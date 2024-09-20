package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

type model struct {
	items         []string
	selected      int
	isPlaying     bool
	closingSignal chan struct{}
	isPaused      bool
	pauseSignal   chan struct{}
	volume        int
	volUp         chan struct{}
	volDown       chan struct{}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "k":
			if m.selected > 0 {
				m.selected--
			}

		case "j":
			if m.selected < len(m.items)-1 {
				m.selected++
			}

		case "enter":
			if m.isPlaying {
				m.closingSignal <- struct{}{} // Stop currently playing music
			}
			m.isPlaying = true
			go m.playMusic(m.items[m.selected])
		case "p":
			m.pauseSignal <- struct{}{}
			m.isPaused = !m.isPaused

		case "u":
			m.volUp <- struct{}{}
			m.volume += 1

		case "d":
			m.volDown <- struct{}{}
			m.volume -= 1
		}
	}

	return m, nil
}

func (m model) View() string {
	s := "Choose a song:\n\n"

	for i, item := range m.items {
		cursor := " " // no cursor
		if m.selected == i {
			cursor = ">" // current cursor
		}
		s += fmt.Sprintf("%s %s\n", cursor, item)
	}

	if !m.isPaused && m.isPlaying {
		s += "\n[Playing]... press \"p\" to pause\n"
	} else if !m.isPlaying {
		s += "\n[Not Started]... press \"<Enter>\" to start\n"
	} else {
		s += "\n[Paused]... press \"p\" to play\n"
	}

	s += fmt.Sprintf("\nVolume  :%v\n", m.volume)
	s += "Vol+ : 'u', Vol- : 'd'\n"
	s += "\nPress 'q' to quit.\n"
	return s
}

func (m *model) playMusic(filepath string) {
	// Load music file
	f, err := os.Open(filepath)
	if err != nil {
		fmt.Println("Could not open file:", err)
		return
	}
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		fmt.Println("Could not decode file:", err)
		return
	}
	defer streamer.Close()

	// Initialize the speaker with the sample rate
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	/*WARNING: When the song naturally ends, the for..select{} section seems to freesze the program, don't know why.
	....For now, I've run the song in an infinite loop (😵)*/
	ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, streamer), Paused: false}
	volume := &effects.Volume{
		Streamer: ctrl,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}

	done := make(chan bool)

	/*FIXME: here this sequence will never go to the next song
	.....beacause it's running a loop,*/
	speaker.Play(beep.Seq(volume, beep.Callback(func() {
		done <- true
	})))

	for {
		select {
		case <-m.pauseSignal:
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()

		case <-m.volUp:
			speaker.Lock()
			volume.Volume += 0.5
			speaker.Unlock()

		case <-m.volDown:
			speaker.Lock()
			volume.Volume -= 0.5
			speaker.Unlock()

		case <-done:
			speaker.Clear()
			m.isPlaying = false
			return
		case <-m.closingSignal:
			speaker.Clear()
			m.isPlaying = false
			return
		}

	}
}

func main() {
	items, err := filepath.Glob("*.mp3")
	if err != nil {
		log.Fatal("error listing files")
	}

	m := model{
		items:         items,
		closingSignal: make(chan struct{}),
		isPlaying:     false,
		pauseSignal:   make(chan struct{}), // unbuffered channel
		isPaused:      false,
		volUp:         make(chan struct{}), // unbuffered channel
		volDown:       make(chan struct{}), // unbuffered channel
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
