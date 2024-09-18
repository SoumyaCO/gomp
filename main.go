package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

type model struct {
	items         []string
	selected      int
	isPlaying     bool
	closingSignal chan struct{}
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

	if m.isPlaying {
		s += "\nPlaying music...\n"
	}

	s += "\nPress q to quit.\n"
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

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	select {
	case <-done:
		m.isPlaying = false
	case <-m.closingSignal:
		speaker.Clear()
		m.isPlaying = false
	}
}

func main() {
	items, err := filepath.Glob("*.mp3")
	if err != nil {
		log.Fatal("error listing files")
	}

	m := model{
		items:     items,
		done:      make(chan struct{}),
		isPlaying: false,
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
