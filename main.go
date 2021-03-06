package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sethgrid/curse"
)

var (
	// sensible defaults to be overridden when we read in screen dimensions
	UID          string = "foo"
	WIDTH        int    = 60
	HEIGHT       int    = 15
	CONSOLE_MODE bool   = false
	PREV_COMMAND string = ""
	COMMAND      string = ""
)

const (
	ESC = 27
	DEL = 127
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
	HEIGHT = screenY - 1 - 3 // 3 lines for COMMAND input
	WIDTH = screenX - 1

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
			if CONSOLE_MODE {
				if string(command) == ";" || command == '\r' {
					processCommand()
				} else if command == '\b' || command == DEL {
					if len(COMMAND) > 1 {
						COMMAND = COMMAND[:len(COMMAND)-1]
					} else {
						COMMAND = ""
					}
				} else {
					COMMAND += string(command)
				}
			} else {
				// todo: use switch
				if string(command) == ":" || command == ESC {
					CONSOLE_MODE = true
				} else if string(command) == "w" {
					sendCommand("mw")
				} else if string(command) == "s" {
					sendCommand("ms")
				} else if string(command) == "a" {
					sendCommand("ma")
				} else if string(command) == "d" {
					sendCommand("md")
				} else if string(command) == "." {
					COMMAND = PREV_COMMAND
					processCommand()
				} else if string(command) == "x" {
					COMMAND = "attack"
					processCommand()
				} else if string(command) == "q" {
					sendCommand("q")
					s.cursor.Move(1, 1)
					s.cursor.SetDefaultStyle()
					s.cursor.EraseAll()
					close(quit)
					break
				} else {
					// log.Fatal(command)
				}
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

func processCommand() {
	if COMMAND == "resize" {
		screenX, screenY, err := curse.GetScreenDimensions()
		if err != nil {
			log.Fatal("unable to get screen dimensions - ", err)
		}
		// todo - dry this up
		HEIGHT = screenY - 1 - 3 // 3 lines for COMMAND input
		WIDTH = screenX - 1
		COMMAND = fmt.Sprintf("resize %d %d", WIDTH, HEIGHT)
	}

	COMMAND = strings.Replace(COMMAND, " (ok)", "", -1)
	COMMAND = strings.Replace(COMMAND, " (not ok)", "", -1)

	success, message := sendCommand(">" + COMMAND)

	suffix := " (not ok)"
	if success {
		suffix = " (ok)"
	}

	if message != "" {
		suffix = " - " + message
	}

	PREV_COMMAND = COMMAND + suffix
	COMMAND = ""
	CONSOLE_MODE = false
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
	defer resp.Body.Close()
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
	fmt.Println(strings.Repeat("=", WIDTH))
	fmt.Println("\r q to quit. w,a,s,d to move. Press `:` to enter command mode.")
	fmt.Println("\r", PREV_COMMAND)
	if CONSOLE_MODE {
		fmt.Printf("\r>:%s", COMMAND)
	} else {
		fmt.Printf("\r>")
	}
}

func sendCommand(cmd string) (bool, string) {
	cmd = url.QueryEscape(cmd)
	resp, err := http.Get("http://localhost:8888/cmd?uid=" + UID + "&key=" + cmd)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	// todo: handle err
	message, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return resp.StatusCode == http.StatusOK, string(message)
}
