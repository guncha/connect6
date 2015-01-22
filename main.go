package main

import (
	"log"
	"net/http"
	//	"fmt"
	// "encoding/json"
	"flag"
	// "math/rand"
	// "os"
	// "strconv"
	// "strings"
	// "time"
)

var addr *string = flag.String("a", ":8080", "address and port to bind server to")

func main() {

	http.Handle("/", http.FileServer(http.Dir(".")))
	log.Println("Application launching on", *addr)

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}
