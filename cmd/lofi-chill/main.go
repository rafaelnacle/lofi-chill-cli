// Command lofi-chill runs a local Pomodoro, radio, and ambience TUI.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"lofi-chill/internal/audio"
	"lofi-chill/internal/config"
	"lofi-chill/internal/ui"
)

// main exits only after run has released its terminal and audio resources.
func main() { os.Exit(run()) }

// run handles CLI flags and the interactive session, returning a process exit code.
func run() int {
	flags := flag.NewFlagSet("lofi-chill", flag.ContinueOnError)
	version := flags.Bool("version", false, "mostrar versão")
	doctor := flags.Bool("doctor", false, "verificar dependências de áudio")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), "lofi & chill — Pomodoro, rádio e sons ambientes no terminal.\n\nUso: lofi-chill [opções]\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		flags.Usage()
		return 2
	}
	if *version {
		fmt.Println("lofi-chill 0.1.0")
		return 0
	}
	if *doctor {
		code := 0
		fmt.Println("Plataforma:", runtime.GOOS)
		for _, name := range []string{"mpv", "yt-dlp"} {
			p, e := exec.LookPath(name)
			if e != nil {
				fmt.Printf("%s: ausente\n", name)
				code = 1
			} else {
				fmt.Printf("%s: %s\n", name, p)
			}
		}
		fmt.Println("Mixer e aviso: mpv. Rádio: mpv + yt-dlp. Pomodoro funciona sem áudio.")
		return code
	}
	path, err := config.Path()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	c, warning := config.Load(path)
	engine := audio.New()
	defer engine.Close()
	result, err := tea.NewProgram(ui.New(c, path, engine, warning), tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Não foi possível abrir a TUI:", err)
		return 1
	}
	if m, ok := result.(ui.Model); ok {
		if err = m.SaveOnExit(); err != nil {
			fmt.Fprintln(os.Stderr, "Não foi possível salvar:", err)
			return 1
		}
	}
	return 0
}
