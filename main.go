package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type listKeyMap struct {
	volumeUp   key.Binding
	volumeDown key.Binding
	playPause  key.Binding
	selectItem key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		volumeUp: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "incr. Volume"),
		),
		volumeDown: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "decr. Volume"),
		),
		playPause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "Play/Pause"),
		),
		selectItem: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "Star Playing a song"),
		),
	}
}

type model struct {
	list          list.Model
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
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			song := m.list.SelectedItem().(item).Title()
			if m.isPlaying {
				m.closingSignal <- struct{}{} // Stop currently playing music
			}
			m.isPlaying = true
			go m.playMusic(song)
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
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
	// if !m.isPaused && m.isPlaying {
	// 	s += "\n[Playing]... press \"p\" to pause\n"
	// } else if !m.isPlaying {
	// 	s += "\n[Not Started]... press \"<Enter>\" to start\n"
	// } else {
	// 	s += "\n[Paused]... press \"p\" to play\n"
	// }
	//
	// s += fmt.Sprintf("\nVolume  :%v\n", m.volume)
	// s += "Vol+ : 'u', Vol- : 'd'\n"
	// s += "\nPress 'q' to quit.\n"
	// return s
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
	fileNames, err := filepath.Glob("*.mp3")
	if err != nil {
		log.Fatal("error listing files")
	}
	var itemList []list.Item
	for _, fileName := range fileNames {
		itemList = append(itemList, item{title: fileName, desc: "A random Song"})
	}

	m := model{
		list:          list.New(itemList, list.NewDefaultDelegate(), 0, 0),
		closingSignal: make(chan struct{}),
		isPlaying:     false,
		pauseSignal:   make(chan struct{}), // unbuffered channel
		isPaused:      false,
		volUp:         make(chan struct{}), // unbuffered channel
		volDown:       make(chan struct{}), // unbuffered channel
	}

	listkeys := newListKeyMap()

	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listkeys.playPause,
			listkeys.volumeUp,
			listkeys.volumeDown,
			listkeys.selectItem,
		}
	}

	/* FIXME: Problem in filterin, when filtering and there is no item with the name, then it panics, and returns, also freezing for somehow, So for now it's DISABLED.*/
	m.list.SetFilteringEnabled(false)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
