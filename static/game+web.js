function randomString( length ) {
	var chars = "abcdefghiklmnopqrstuvwxyz";
	var string_length = length || 10
	var randomstring = "";
	for (var i=0; i<string_length; i++) {
		var rnum = Math.floor(Math.random() * chars.length);
		randomstring += chars.substring(rnum,rnum+1);
	}
	return randomstring
};

$(document).ready(function() {
	
	// todo: convert all those grids to class coords
	// and implement coords.isEqual()	
	
	var GameObject = {
		myName: "",
		myColor: "black",
		myGame: "",
		stones: {
			image: new Image(),
			array: [],
			draw: function( ctx, grid, color ) {
				if ( grid && color ) {
					
					var x = grid.X*30+15;
					var y = grid.Y*30+15;
					
					// draw one
					if ( color == "white") {
						ctx.drawImage( this.image, 0, 0, 30, 30, x, y, 30, 30);
					} else {
						ctx.drawImage( this.image, 0, 30, 30, 30, x, y, 30, 30);					
					}
				} else {
					
					// draw all
					ctx.clearRect(0, 0, 600, 600);
					for (var i = 0, j = this.array.length; i < j; i++) {
						var o = this.array[i];
						this.draw( ctx, o.grid, o.color);
					}
				}
			},
			place: function( grid, color ) {
				if ( ! this.isPlaced( grid ) ) {
										
					this.array.push( { grid: grid, color: color} );
				}
			},
			isPlaced: function( grid ) {
				for (var i = 0, j = this.array.length; i < j; i++) {
					if ( this.array[i].grid.X == grid.X && this.array[i].grid.Y == grid.Y) {
						return true;
					}
				}
				return false;
			}
		},
		ctx: $("#connect6").get(0).getContext("2d"),
		placeSound: null,
		gameInProgress: false,
		oldPotentialMove: {
			X: 0,
			Y: 0
		},
		drawPotentialMove: function( grid ) {
			this.ctx.save();
			this.ctx.globalAlpha = 0.35;
			this.stones.draw( this.ctx, grid, this.myColor);
			this.ctx.restore();
		},
		drawClearGrid: function( grid ) {
			var c = this.gridToPixels( grid );
			this.ctx.clearRect(c.X, c.Y, 30, 30);
		},
		gridToPixels: function( c ) {
			return { X: c.X * 30 + 15, Y: c.Y * 30 + 15 };
		},
		pixelsToGrid: function( c ) {
			return { X: parseInt((c.X - 15)/ 30), Y: parseInt((c.Y - 15) / 30) };
		},
		mouseMove: function( c ) {
			
			if ( c.X < 15 || c.X > 575 || c.Y < 15 || c.Y > 575){
				return
			}

			var grid = this.pixelsToGrid( c );

			if ( grid.X != this.oldPotentialMove.X ||
					grid.Y != this.oldPotentialMove.Y ) {
				
				if ( ! this.stones.isPlaced( this.oldPotentialMove ) ) {
					this.drawClearGrid( this.oldPotentialMove );
				}
				if ( this.isPlayersTurn() && this.gameInProgress ) {
					if ( ! this.stones.isPlaced( grid ) && this.isValidGrid( grid )) {
						this.oldPotentialMove = grid;
						this.drawPotentialMove( grid );
					}
				}
			};
		},
		mouseClick: function( c ) {
			
			if ( ! this.isPlayersTurn() || ! this.gameInProgress ) {
				return
			}
			
			var grid = this.pixelsToGrid( c );
			
			if ( ! this.isValidGrid( grid )) {
				return
			}
			
			if ( ! this.stones.isPlaced( grid ) ) {
						
				this.postToServer( grid, this.stones.array.length );
				this.stones.place( grid, this.myColor);
				this.stones.draw( this.ctx, grid, this.myColor);
				this.displayStatus();
				
				var v = this.isVictorious();

				if ( v ) {

					this.endGame();

				}			
			}
		},
		receiveMove: function( data ) {
			var move = {
				grid: data.Coords,
				color: this.getColorFromMovenum( data.Movenum )
			};

			this.stones.array[data.Movenum] = move;
			this.stones.draw( this.ctx, move.grid, move.color );
			
			var v = this.isVictorious();
			
			if ( v ) {
				
				this.endGame();
				
			}
		},
		startGame: function() {
			this.gameInProgress = true;
			this.getMovesPoll();
			
			$("#status").html("Game on!");
			
			if ( this.myColor == "black" ) {
				$("#status").append(" Your turn.");
			} else {
				$("#status").append(" Wait for your turn.");
			}						
		},
		endGame: function() {			
			
			this.gameInProgress = false;
			this.animateVictorious();
			this.myGame = null;
			this.myColor = null;
		},
		displayStatus: function() {
			
			if ( ! this.gameInProgress ) {
				return
			}
			
			if ( this.isPlayersTurn() ) {
				$("#status").html("YOUR MOVE.");
			} else {
				// some wierd last word selected bug in chrome
				$("#status").html("OPPONENT'S MOVE.");
			}
		},
		getStrangerPoll: function() {
			
			$.ajax({
				type: "GET",
				url: "/game/stranger",
				cache: false,
				context: GameObject,
				data: {name: this.myName},
				error: function(r, status) {
					if ( status == "timeout") {
						this.getStrangerPoll();
					}
				},
				success: function( data ) {
					if ( data.Error ) {
						
						tmp = function() {
							GameObject.getStrangerPoll();
						}
						
						if ( parseInt(data.Timeout) ) {
							setTimeout(tmp, parseInt(data.Timeout));
						}
						
						return;
					}
					
					if ( data.black == this.myName ) {
						this.myColor = "black";
					} else {
						this.myColor = "white";
					}
					this.myGame = data.gameid
					
					this.startGame();
				},
				timeout: 10000 // could be much more..
			});
		},
		getMovesPoll: function() {
			
			if ( !this.myGame || ! this.myName ) {
				return
			}
			
			$.ajax({
				type: "GET",
				url: "/game/"+this.myGame+"/"+this.myName,
				cache: false,
				context: GameObject,
				error: function(r, status) {
					if ( status == "timeout") {
						this.getMovesPoll();
					} else {
						tmp = function() {
							GameObject.getMovesPoll();
						}
						setTimeout(tmp, 5000);
					}
				},
				success: function( data ) {
//					$("#response").append( JSON.stringify(data) + "<br />");
					
					if ( data.Error ) {
						
						tmp = function() {
							GameObject.getMovesPoll();
						}
						
						if ( parseInt(data.Timeout) ) {
							setTimeout(tmp, parseInt(data.Timeout));
						}
						
						return;
						
					} else {
						
						this.receiveMove( data );
						this.displayStatus();
						
					};
					
					// fetch next one
					this.getMovesPoll();
				},
				timeout: 50000
			});			
		},
		animateVictorious: function() {
			// only call after .isVictorious()
			
			var v = this.isVictorious();
			var root = v.grid;
								
			if ( v.color == this.myColor ) {
				$("#status").html("Game over. Well played!");
			} else {
				$("#status").html("Oh, noes. I was rooting for you.");
			}
			
			// if "a" contains grid, append grid to "b"
			var containAppend = function(a, b, grid) {
				for (var i = 0, j = a.length; i < j; i++) {
					if ( a[i].grid.X == grid.X && a[i].grid.Y == grid.Y ) {
						b.push( grid );
					}
				}
				return false;
			}
			
			var b = [[],[],[],[]];
			for (var i=0; i < 6; i++){
				containAppend( this.stones.array, b[0], {X:root.X+i, Y:root.Y} );
				containAppend( this.stones.array, b[1], {X:root.X+i, Y:root.Y+i} );
				containAppend( this.stones.array, b[2], {X:root.X, Y:root.Y+i} );
				containAppend( this.stones.array, b[3], {X:root.X-i, Y:root.Y+i} );
			}
			
			for (var i=0; i < 4; i++) {
				if (b[i].length == 6) {
					var s = b[i];
				}
			}
			
			animate_clear = function() {
				for (var i=0; i < 6; i++) {
					GameObject.drawClearGrid( s[i] );
				}
			}
			animate_draw = function() {
				GameObject.stones.draw( GameObject.ctx );
			}
			
			show_reset = function() {
				$("#chooser").toggle( true );
				$("#chooser li:eq(0)").toggle( false );
				$("#chooser li:eq(1)").toggle( false );
				$("#chooser li:eq(2)").toggle( true );
			}
			
			for (var i=0; i < 5; i++) {
				setTimeout( animate_clear, i * 1000 );
				setTimeout( animate_draw, i * 1000 + 500 );
			}
			
			setTimeout( show_reset, (i+1) * 1000);
			
		},
		isVictorious: function() {
			
			var a = new Array();
			for ( var x = 0; x < 19; x++) {
				var b = new Array();
				for ( var y = 0; y < 19; y++) {
					b[y] = 0;
				}
				a.push(b);
			}
			
			// 0 empty
			// 1 black
			// 2 white
			
			for (var i = 0, j = this.stones.array.length; i < j; i++) {
				var o = this.stones.array[i];
				a[o.grid.X][o.grid.Y] = (o.color == "black") ? 1 : 2;
			}
			
			var isSame = function(x, y, tx, ty) {
				for (var i = 1; i < 6; i++) {
					if ( a[x+i*tx][y+i*ty] != a[x][y] ) {
						return false;
					}
				}
				
				return true;
			};
			
			var grid
			
			for ( var x = 0; x < 19; x++) {
				for ( var y = 0; y < 19; y++) {
					if ( a[x][y] == 0 ) {
						continue;
					}
					if ( x < 14 && y < 14) {
						if ( isSame(x, y, 1, 0) || isSame(x, y, 1, 1) || isSame(x, y, 0, 1) ) {
							return {
								grid: { X: x, Y: y},
								color: ( a[x][y] == 1 ) ? "black" : "white"
							}
						}
					}
					if ( x > 4 && y < 14) {
						if ( isSame(x, y, -1, 1) ) {
							return {
								grid: { X: x, Y: y},
								color: ( a[x][y] == 1 ) ? "black" : "white"
							}
						}
					}
				}
			}
			
			return false;
		},
		postToServer: function( grid, movenum ) {
			// pray to gods that this doesnt fail
			
			if ( this.myName && this.myGame ) {
				
				var o = {
					Gameid: parseInt( this.myGame ),
					Movenum: movenum,
					Player: this.myName,
					Coords: grid
				};
				
				$.post("/game/"+this.myGame, {data: JSON.stringify(o)} );
			}
		},
		isPlayersTurn: function( color ) {
			
			var player = color || this.myColor;
			var movenum = this.stones.array.length;
			
			if ( player == this.getColorFromMovenum( movenum )) {
				return true;
			} else {
				return false;
			}
		},
		getColorFromMovenum: function( movenum ) {

			switch ( movenum % 4) {
				
				case 0: return "black";
				case 1: return "white";
				case 2: return "white";
				case 3: return "black";
			}		
		},

		isValidGrid: function( grid ) {
			return grid.X >= 0 && grid.X <= 18 && grid.Y >= 0 && grid.Y <= 18;
		},
	};

	if( window.location.hash ) {
		GameObject.myGame = window.location.hash.substring(1);
		window.location.hash = "";
	}
	
	var stones = new Image();
	stones.src = "static/stones.png"
	
	stones.onload = function(){
		
		GameObject.stones.image = this;
	}
	
/*	if( !!document.createElement('audio').canPlayType ) {
		
		$("#container").after('\
		<audio id="placesound" preload="auto">\
		      <source src="/static/puk.ogg" type="audio/ogg"></source>\
		      <source src="/static/puk.mp3" type="audio/mpeg"></source>\
		      <source src="/static/puk.wav" type="audio/x-wav"></source>\
	    </audio>\
		');
		
		GameObject.placeSound = $("#placesound").get(0);
		GameObject.placeSound.play();
	}
*/	
	var thisname = randomString(10);
	GameObject.myName = thisname;
	
	$("#connect6").mousemove(function(e){
		
		// this.offsetLeft doesn't work on relative elements
		var p = $("#container").offset();
		
		var x = e.pageX - this.offsetLeft - p.left;
		var y = e.pageY - this.offsetTop - p.top;
		
		GameObject.mouseMove( { X: x, Y:y } );
	});
	
	$("#connect6").click(function(e) {
		
		var p = $("#container").offset();
				
		var x = e.pageX - this.offsetLeft - p.left;
		var y = e.pageY - this.offsetTop - p.top;
		
		GameObject.mouseClick( { X: x, Y:y } );
	});
	
	$("#links > a:eq(0)").click( function(){
		$("#info").toggle( false );
		$("#rules").toggle();
	});
	$("#links a:eq(1)").click( function(){
		$("#rules").toggle( false );
		$("#info").toggle();
	});
	
	// friend
	$("#chooser li:eq(0)").click( function() {
		alert("Not implemented");
	});
	
	// stranger
	$("#chooser li:eq(1)").click( function() {
		$("#chooser").toggle( false );
		$("#status").html("Waiting for a stranger..");
		GameObject.getStrangerPoll();
	});
	
	$("#chooser li:eq(0)").toggle( false );
	$("#chooser li:eq(2)").toggle( false );	
	
	// reset
	$("#chooser li:eq(2)").click( function() {
//		$("#chooser li:eq(0)").toggle( true );
		$("#chooser li:eq(1)").toggle( true );
		$("#chooser li:eq(2)").toggle( false );
		GameObject.stones.array = [];
		GameObject.stones.draw( GameObject.ctx );
	});	
	
});