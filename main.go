package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sethgrid/curse"
)

var (
	// sensible defaults to be overridden when we read in screen dimensions
	UID    string = "foo"
	WIDTH  int    = 60
	HEIGHT int    = 15
)

type screen struct {
	width, height int
	surface       map[string]*position
	cursor        *curse.Cursor
}

type position struct {
	backgroundColor int
	color           int
	character       rune
}

func init() {
	flag.StringVar(&UID, "uid", "foo", "the user uid (uid) passed to server")
}

func main() {
	flag.Parse()
	var err error

	_, err = curse.New()
	if err != nil {
		log.Fatal("unable to initialize curse environment - ", err)
	}

	screenX, screenY, err := curse.GetScreenDimensions()
	if err != nil {
		log.Fatal("unable to get screen dimensions - ", err)
	}
	s := NewScreen(screenX, screenY)
	HEIGHT = screenY
	WIDTH = screenX

	s.cursor.ModeRaw()
	defer s.cursor.ModeRestore()
	input := bufio.NewReader(os.Stdin)

	quit := make(chan int)

	go func() {
		for {
			command, err := input.ReadByte()
			if err != nil {
				fmt.Println(err)
			}
			if string(command) == "w" {
				sendCommand("mw")
			} else if string(command) == "s" {
				sendCommand("ms")
			} else if string(command) == "a" {
				sendCommand("ma")
			} else if string(command) == "d" {
				sendCommand("md")
			} else if string(command) == "q" {
				sendCommand("q")
				s.cursor.Move(1, 1)
				s.cursor.SetDefaultStyle()
				s.cursor.EraseAll()
				close(quit)
				break
			}
		}
	}()

	go func() {
		for _ = range time.Tick(time.Millisecond * 100) {
			s.Paint()
		}
	}()

	<-quit
}

func NewScreen(x, y int) *screen {
	newScreen := &screen{width: x, height: y}
	newScreen.surface = make(map[string]*position)
	newScreen.cursor, _ = curse.New()
	return newScreen
}

func (s *screen) Paint() {
	s.cursor.Move(1, 1)
	s.cursor.EraseDown()
	s.cursor.SetBackgroundColor(curse.WHITE).SetColor(curse.BLACK)

	resp, err := http.Get(fmt.Sprintf("http://localhost:8888?uid=%s&w=%d&h=%d", UID, WIDTH, HEIGHT))
	if err != nil {
		fmt.Println("Server Connection Lost - ", err)
		return
	}
	reader := bufio.NewReader(resp.Body)

	for {
		r, _, err := reader.ReadRune()
		if err != nil && err != io.EOF {
			log.Fatal("error reading rune", err)
		}
		fmt.Printf("%c", r)
		if r == '\n' {
			fmt.Printf("\r")
		}
		if err == io.EOF {
			break
		}
	}
}

func sendCommand(cmd string) {
	http.Get("http://localhost:8888/cmd?uid=" + UID + "&key=" + cmd)
}
