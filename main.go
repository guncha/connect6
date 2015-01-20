package main

import (
	"log"
	"net/http"
	//	"fmt"
	"encoding/json"
	"flag"
	"github.com/petar/GoLLRB/llrb"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// proposed data structures
type coords struct {
	X int
	Y int
}

func (a *coords) isEqual(b *coords) bool {

	return a.X == b.X && a.Y == b.Y
}

type game struct {
	id           int
	ended        bool
	movesDone    int
	lastActivity int64
	playerBlack  string
	playerWhite  string

	// white player is listening on blackChan
	blackChan chan move
	whiteChan chan move

	moves []coords
}

func (a *game) Less(b llrb.Item) bool {
	return a.id < (b.(*game)).id
}

func newGame() (o *game) {
	o = &game{}
	o.blackChan = make(chan move, 2) // these have to be buffered
	o.whiteChan = make(chan move, 2)
	return
}
func (g *game) getMoveColor(move int) (c string) {

	// 0 is black
	// 1 is white
	// 2 is white
	// 3 is black
	// 4 is black
	// etc..

	switch move % 4 {

	case 0:
		c = "black"
	case 1:
		c = "white"
	case 2:
		c = "white"
	case 3:
		c = "black"
	}

	return

}
func (g *game) isValidMove(m move) (bool, string) {

	// gotta catch em' all
	// otherwise things will go to shit

	if g.movesDone != m.Movenum {

		return false, "invalid movenum"
	}

	// check overlap
	for _, c := range g.moves {

		if c.isEqual(&m.Coords) {

			return false, "move already made"
		}
	}

	// check if it's players turn
	b1 := m.Player == g.playerWhite
	b2 := m.Player == g.playerBlack
	b3 := g.getMoveColor(m.Movenum) == "white"
	b4 := g.getMoveColor(m.Movenum) == "black"

	switch {

	case b1 && b4:
		return false, "not player's turn"
	case b2 && b3:
		return false, "not player's turn"
	case !b1 && !b2:
		return false, "not in the game"
	}

	return true, "OK"
}

type move struct {
	Gameid  int
	Movenum int
	Player  string
	Coords  coords
}
type jsonReturn struct {
	Error   bool
	Timeout int // 0 for fatal, t>0 in ms
	Message string
}
type message struct {
	request    string
	reply_chan chan message
	data       interface{}
}
type webHandler struct {
	moveChan chan message
	dbChan   chan message
}

func (wh *webHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// calls apropriate function depending on request
	// possible requests:
	// POST /game/1234 - post move to db, player name in POST params
	// GET /game/1234/somename - long polls for opponent's moves
	// GET /game/stranger - long polls for opponent to play with
	// GET /game/friend?name=b - sets up new game
	// GET /game/join?name=a&game=1234 - adds player to game
	// GET /game/1234 - redirects to /#1234 where client joint said game

	defer func() {

		if r := recover(); r != nil {
			log.Println("ServeHTTP: panic:", r)
		}
	}()

	if req.Method == "POST" {

		// player is making a move
		webPostMove(w, req, wh.moveChan)

	} else if req.Method == "GET" {

		switch req.URL.Path {
		case "/game/stranger":
			webPollStranger(w, req, wh.dbChan)
		case "/game/friend":
			webFriendGame(w, req, wh.dbChan)
		case "/game/join":
			webJoinGame(w, req, wh.dbChan)
		default:
			{

				params := strings.Split(req.URL.Path, "/")

				if len(params) == 3 {
					// eg: GET /game/1234

					log.Println("redirekto..")
					http.Redirect(w, req, "/#"+params[2], 302)

				} else {

					webPushMove(w, req, wh.dbChan)
				}
			}
		}
	}
}

type playerWaiting struct {
	name     string
	ch       chan message
	lastSeen int64
}

func (p *playerWaiting) reset() {
	p.name = ""
	p.ch = nil
}

func gameController(moveChan chan message, dbChan chan message) {
	// recieves moves, checks for validity and game conditions and saves

	// this is reused
	var msg message

	var errorHandler = func(e string, c chan message) {

		// errors are fed backwards to worker thread,
		// that feeds them to webclient and also prints

		var msg message
		msg.request = "error"
		msg.reply_chan = nil
		msg.data = "gameController: " + e

		c <- msg

		return
	}

	for moveMessage := range moveChan {

		m := moveMessage.data.(move)
		worker_chan := moveMessage.reply_chan

		log.Println("gameController: got move:", m)

		msg.request = "get_game"
		msg.reply_chan = make(chan message)
		msg.data = m.Gameid

		dbChan <- msg
		msg = <-msg.reply_chan

		var g *game

		if msg.request == "found" {

			g = msg.data.(*game)

		} else {

			errorHandler("no such game", worker_chan)
			continue
		}

		if b, err := g.isValidMove(m); !b {

			errorHandler(err, worker_chan)
			continue
		}

		g.moves = append(g.moves, m.Coords)
		g.movesDone++

		// check for victory condition
		// if true, set g.ended true

		// push move to other player
		// check if channel is full to prevent blocking
		if g.playerBlack == m.Player {

			if len(g.blackChan) > 1 {

				errorHandler("blackChan already full (?)", worker_chan)
				continue
			}

			g.blackChan <- m

		} else {

			if len(g.blackChan) > 1 {

				errorHandler("whiteChan already full (?)", worker_chan)
				continue
			}

			g.whiteChan <- m
		}

		// save game to db
		msg.request = "put_game"
		msg.reply_chan = nil
		msg.data = g

		dbChan <- msg

		// signal web worker
		msg.request = "OK"
		worker_chan <- msg

	}
}
func dbLookupGame(t *llrb.LLRB, gameid int) *game {

	g := &game{id: gameid}
	result := t.Get(g)
	if result != nil {

		g = result.(*game)
		return g
	}

	return nil
}
func dbGetMaxGame(t *llrb.LLRB) int {

	max := t.Max()

	if max == nil {

		return 0
	}

	return max.(*game).id
}
func dbController(dbChan chan message) {

	/*	this function responds to messages on db channel
		possible requests:

		get_game with id in data field, responds with game struct
		get_max_gamenum, replies with largest gamenum in data field
		put_game with game in data, puts game in tree, no reply
		new_stranger with player info in data, creates g and returns

	*/
	tree := llrb.New()
	var reply message
	var stranger playerWaiting

	for m := range dbChan {

		// fmt.Printf("dbController: got message: %#v\n", m)

		switch m.request {

		case "get_game":
			{

				// log.Println("dbController: request for game id ", m.data.(int) )
				g := dbLookupGame(tree, m.data.(int))

				if g != nil {

					reply.request = "found"
				} else {
					reply.request = "not_found"
				}

				reply.reply_chan = nil
				reply.data = g
				m.reply_chan <- reply
			}
		case "put_game":
			{

				// log.Println("dbController: putting game")
				g := m.data.(*game)
				g.lastActivity = time.Now().Unix()
				tree.ReplaceOrInsert(g)
			}
		case "get_max_gamenum":
			{

				reply.data = dbGetMaxGame(tree)
				reply.request = "OK"
				m.reply_chan <- reply
			}
		case "new_stranger":
			{

				// check if stranger has expired
				// client timeout is 10 seconds
				// we should have a new request by now
				if (time.Now().Unix() - stranger.lastSeen) > 10 {

					log.Println("dbController: timed out. resetting stranger{}")
					stranger.reset()
				}

				if stranger.name == "" || stranger.name == m.data.(string) {

					// first player or repeat customer
					stranger.name = m.data.(string)
					stranger.ch = m.reply_chan
					stranger.lastSeen = time.Now().Unix()

					log.Println("dbController: player waiting: ", stranger.name)

				} else {

					players := []string{stranger.name, m.data.(string)}
					first := rand.Int() % 2
					gameid := dbGetMaxGame(tree) + 1
					returnData := map[string]string{
						"gameid": strconv.Itoa(gameid),
						"black":  players[first],
						"white":  players[1-first]}

					g := newGame()
					g.id = gameid
					g.playerBlack = players[first]
					g.playerWhite = players[1-first]
					tree.ReplaceOrInsert(g)

					log.Println("dbController: joining", players[0], players[1], "in game", gameid)

					reply.request = "OK"
					reply.reply_chan = nil
					reply.data = returnData

					stranger.ch <- reply
					m.reply_chan <- reply

					stranger.reset()

				}
			}
		default:
			{
				log.Println("dbController: unknown request: ", m.request)
			}
		}
	}
}
func webPollStranger(w http.ResponseWriter, req *http.Request, dbChan chan message) {

	name := req.FormValue("name")

	if name == "" {

		webErrorHandler("name not passed", w)
		return
	}

	var msg message
	msg.request = "new_stranger"
	msg.reply_chan = make(chan message)
	msg.data = name

	dbChan <- msg

	timeout := time.Tick(52e9) // 52s timeout

	select {

	case msg = <-msg.reply_chan:
	case <-timeout:
		{
			log.Println("webPollStranger: timed out.")
			return
		}
	}
	w.Header().Set("Content-Type", "text/json")

	var b []byte
	if msg.request != "OK" {

		b, _ = json.Marshal(&jsonReturn{Error: true, Timeout: 5000, Message: msg.data.(string)})

	} else {

		b, _ = json.Marshal(msg.data)
	}

	_, err := w.Write(b)

	if err != nil {

		log.Println("webPushMove: writing to connection failed: ", err)
	}

	return
}
func webFriendGame(w http.ResponseWriter, req *http.Request, dbChan chan message) {
	// creates new game, returns JSON with gameid and player color

	name := req.FormValue("name")

	if name == "" {

		webErrorHandler("name not passed", w)
		return
	}

	var msg message
	msg.request = "get_max_gamenum"
	msg.reply_chan = make(chan message)
	msg.data = nil

	dbChan <- msg
	msg = <-msg.reply_chan

	g := newGame()
	g.id = msg.data.(int) + 1

	switch rand.Int() % 2 {
	case 0:
		g.playerBlack = name
	case 1:
		g.playerWhite = name
	}

	msg.request = "put_game"
	msg.reply_chan = nil
	msg.data = g

	dbChan <- msg
	w.Header().Set("Content-Type", "text/json")

	b, _ := json.Marshal(map[string]string{
		"gameid": strconv.Itoa(g.id),
		"black":  g.playerBlack,
		"white":  g.playerWhite})

	if _, err := w.Write(b); err != nil {

		log.Println("webFriendGame: writing to connection failed: ", err)
	}

	return
}
func webJoinGame(w http.ResponseWriter, req *http.Request, dbChan chan message) {
	// adds player to db or returns error messages to display in client's status bar

	name := req.FormValue("name")
	gameIdString := req.FormValue("game")

	gameid, err := strconv.Atoi(gameIdString)

	if name == "" || gameIdString == "" || err != nil {

		webErrorHandler("name or game not passed or invalid", w)
		return
	}

	var msg message
	msg.request = "get_game"
	msg.reply_chan = make(chan message)
	msg.data = gameid

	dbChan <- msg
	msg = <-msg.reply_chan

	if msg.request != "found" {

		webErrorHandler("The game with id "+gameIdString+" does not exist. Yet.", w)
		return
	}

	g := msg.data.(*game)

	switch {
	case g.playerBlack == "":
		g.playerBlack = name
	case g.playerWhite == "":
		g.playerWhite = name
	default:
		{
			webErrorHandler("The game is already full.", w)
			return
		}
	}

	msg.request = "put_game"
	msg.reply_chan = nil
	msg.data = g

	dbChan <- msg
	w.Header().Set("Content-Type", "text/json")

	b, _ := json.Marshal(map[string]string{
		"gameid": gameIdString,
		"black":  g.playerBlack,
		"white":  g.playerWhite})

	if _, err := w.Write(b); err != nil {

		log.Println("webJoinGame: writing to connection failed: ", err)
	}

	return
}
func webErrorHandler(e string, w http.ResponseWriter) {

	log.Println("webxxxxMove:", e)

	jr := &jsonReturn{Error: true, Message: e}
	b, _ := json.Marshal(jr)
	w.Header().Set("Content-Type", "text/json")
	w.Write(b)
	return
}
func webPostMove(w http.ResponseWriter, req *http.Request, moveChan chan message) {

	var m move

	params := strings.Split(req.URL.Path, "/")

	if len(params) < 3 {

		webErrorHandler("Missing GameID in request", w)
		return
	}

	gameid, err := strconv.Atoi(params[2])
	if err != nil {

		webErrorHandler("Invalid GameID: "+err.Error(), w)
		return
	}
	m.Gameid = gameid

	formData := req.FormValue("data")
	if err := json.Unmarshal([]uint8(formData), &m); err != nil {

		webErrorHandler("Can't unmarshal post data: "+err.Error(), w)
		return
	}

	// encapsulate in message and
	// post the move to gameController

	var msg message

	msg.reply_chan = make(chan message)
	msg.data = m
	moveChan <- msg

	msg = <-msg.reply_chan

	if msg.request != "OK" {

		webErrorHandler(msg.data.(string), w)
		return
	}
	w.Header().Set("Content-Type", "text/json")

	b, _ := json.Marshal(&jsonReturn{Message: formData})

	if _, err := w.Write(b); err != nil {

		log.Println("webPostMove: writing to connection failed: ", err)
	}

	return
}
func webPushMove(w http.ResponseWriter, req *http.Request, dbChan chan message) {

	// long polling, JSON reply
	// typical request: /game/12345/playername

	params := strings.Split(req.URL.Path, "/")

	if len(params) < 4 {

		webErrorHandler("Missing GameID/playerName in request", w)
		return
	}

	playerName := params[3]
	gameid, err := strconv.Atoi(params[2])
	if err != nil || len(playerName) == 0 {

		webErrorHandler("Invalid GameID and/or playerName: "+err.Error(), w)
		return
	}

	var msg message
	msg.request = "get_game"
	msg.reply_chan = make(chan message)
	msg.data = gameid
	dbChan <- msg
	msg = <-msg.reply_chan

	if msg.request == "not_found" {

		webErrorHandler("Long-polling for nonexistent game", w)
		return

	}

	g := msg.data.(*game)
	var opponentChan chan move

	switch {

	case playerName == g.playerBlack:
		opponentChan = g.whiteChan
	case playerName == g.playerWhite:
		opponentChan = g.blackChan
	default:
		{

			webErrorHandler("Player not in the game", w)
			return
		}
	}

	timeout := time.Tick(52e9) // 52s timeout
	var m move

	select {

	case m = <-opponentChan:
	case <-timeout:
		{
			log.Println("webPushMove: timed out.")
			return
		}
	}
	w.Header().Set("Content-Type", "text/json")
	b, _ := json.Marshal(m)

	if _, err := w.Write(b); err != nil {

		log.Println("webPushMove: writing to connection failed: ", err)
	}

	return
}
func webStatic(w http.ResponseWriter, req *http.Request) {

	if req.URL.Path == "/static/" {

		http.Redirect(w, req, "../", 301)

	} else {

		//		log.Println("webStatic: serving", req.URL.Path)
		http.ServeFile(w, req, "."+req.URL.Path)
	}
}
func webDefault(w http.ResponseWriter, req *http.Request) {

	if req.URL.Path == "/" {

		http.ServeFile(w, req, "./static/main.html")

	} else {

		http.Redirect(w, req, "/", 301)
	}

}

var addr *string = flag.String("a", ":8080", "address and port to bind server to")
var logfile *string = flag.String("logfile", "-", "file to save log messages, - is stdout")

func main() {

	flag.Parse()

	if *logfile != "-" {

		fi, err := os.Stat(*logfile)
		var logflag int

		if err == nil && fi.Mode().IsRegular() {

			logflag = os.O_APPEND | os.O_WRONLY

		} else {

			logflag = os.O_CREATE | os.O_WRONLY
		}

		// this should somehow be closed as well
		// but that involves handling SIGTERM, that
		// in turn involves handling SIGPIPEs etc
		f, err := os.OpenFile(*logfile, logflag, 0666)

		if err != nil {

			log.Fatal("main: log can't create/append to ", *logfile)

		} else {
			log.SetOutput(f)
			log.Println("--- begin logging session ---")
		}

	}

	var wh webHandler

	// this should probably be buffered
	wh.moveChan = make(chan message)
	wh.dbChan = make(chan message)

	http.Handle("/game/", &wh)
	http.HandleFunc("/static/", webStatic)
	http.HandleFunc("/", webDefault)

	// these run in a seperate goroutines
	go gameController(wh.moveChan, wh.dbChan)
	go dbController(wh.dbChan)

	log.Println("Application launching on ", *addr)

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}
