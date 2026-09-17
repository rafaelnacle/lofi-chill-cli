package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lofi-chill/internal/audio"
	"math"
	"strings"
)

// bigClock draws decimal digits and a colon as a three-line display.
func bigClock(value string) string {
	glyphs := map[rune][3]string{
		'0': {"█▀█", "█ █", "█▄█"},
		'1': {" ▀█", "  █", "  █"},
		'2': {"▀▀█", "█▀▀", "█▄▄"},
		'3': {"▀▀█", " ▀█", "▄▄█"},
		'4': {"█ █", "▀▀█", "  █"},
		'5': {"█▀▀", "▀▀█", "▄▄█"},
		'6': {"█▀▀", "█▀█", "█▄█"},
		'7': {"▀▀█", "  █", "  █"},
		'8': {"█▀█", "█▀█", "█▄█"},
		'9': {"█▀█", "▀▀█", "▄▄█"},
		':': {" ▄ ", "   ", " ▀ "},
	}
	rows := [3]string{}
	for _, digit := range value {
		for row := range rows {
			rows[row] += glyphs[digit][row] + " "
		}
	}
	return strings.Join(rows[:], "\n")
}

// clockText keeps an ordinary numeric countdown visible on every panel.
func (m Model) clockText() string {
	seconds := int(math.Ceil(m.timer.Sessions[m.timer.Mode].Remaining.Seconds()))
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// timerState distinguishes new, running, and paused sessions without color.
func (m Model) timerState() string {
	if !m.timer.Deadline.IsZero() {
		return "Em andamento"
	}
	if m.timer.Sessions[m.timer.Mode].Started {
		return "Pausado"
	}
	return "Pronto"
}

// timerAction names the action performed by the global Space shortcut.
func (m Model) timerAction() string {
	if !m.timer.Deadline.IsZero() {
		return "Pausar"
	}
	if m.timer.Sessions[m.timer.Mode].Started {
		return "Retomar"
	}
	return "Iniciar"
}

// enabled labels preferences consistently in Portuguese.
func enabled(value bool) string {
	if value {
		return "Ligado"
	}
	return "Desligado"
}

// selected marks the keyboard target in text as well as color.
func selected(label string, active bool) string {
	if active {
		return accent.Render("> " + label)
	}
	return "  " + label
}

// items supplies controls in keyboard order for one panel.
func (m Model) items(page int) ([]string, int) {
	switch page {
	case 0:
		return []string{
			"Modo: " + m.timer.Mode.String() + "  [1/2/3]",
			fmt.Sprintf("Duração: %d min  [-/+]", m.timer.Durations[m.timer.Mode]),
			m.timerAction() + " sessão  [espaço]",
			"Reiniciar sessão  [r]",
			"Aviso: " + enabled(m.cfg.Chime) + "  [c]",
			"Testar aviso  [t]",
		}, m.timerRow
	case 1:
		action := "Tocar rádio"
		if m.sound.Radio {
			action = "Parar rádio"
		}
		return []string{
			audio.Stations()[m.sound.Station].Name,
			fmt.Sprintf("Volume: %d%%", m.sound.RadioVolume),
			action + "  [p]",
		}, m.radioRow
	default:
		names := [5]string{"Chuva", "Ruído marrom", "Ondas do mar", "Pássaros", "Lareira"}
		rows := make([]string, 0, 6)
		for i, name := range names {
			v := m.sound.Volumes[i]
			rows = append(rows, fmt.Sprintf("%-12s %s %3d%%", name, strings.Repeat("━", v/10)+strings.Repeat("·", 10-v/10), v))
		}
		action := "Iniciar mixer"
		if m.sound.Mixer {
			action = "Pausar mixer"
		}
		return append(rows, action+"  [m]"), m.channel
	}
}

// panelContent reserves space for selected controls before adding decoration.
func (m Model) panelContent(page, width, height int) string {
	if height <= 0 {
		return ""
	}
	titles := [3]string{"POMODORO", "RÁDIO", "SONS AMBIENTES"}
	title := titles[page]
	if m.page == page {
		title = accent.Render("> " + title + " · selecionado")
	}
	lines := []string{title}
	switch page {
	case 0:
		if height >= 12 && width >= 30 {
			lines = append(lines, "", lipgloss.NewStyle().Foreground(pink).Bold(true).Render(bigClock(m.clockText())))
		}
	case 1:
		lines = append(lines, "Estado: "+m.radioStatus)
	case 2:
		lines = append(lines, "Estado: "+m.mixerStatus)
	}
	prefix := strings.Split(strings.Join(lines, "\n"), "\n")
	rows, cursor := m.items(page)
	available := max(1, height-len(prefix))
	// Small terminals show a window around selection, with explicit position.
	start := max(0, cursor-available+1)
	if len(rows) > available {
		prefix[0] = fmt.Sprintf("%s · %d/%d", titles[page], cursor+1, len(rows))
	}
	for i := start; i < min(len(rows), start+available); i++ {
		prefix = append(prefix, selected(rows[i], m.page == page && i == cursor))
	}
	return fit(strings.Join(prefix, "\n"), width, height)
}

// bodyHeight allocates content after the header, navigation, and fixed footer.
func (m Model) bodyHeight() int { return max(1, m.height-7) }

// box adds a complete border within its exact width and height budget.
func (m Model) box(page, width, height int) string {
	content := m.panelContent(page, width-4, height-2)
	lines := strings.Split(content, "\n")
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	color := purple
	if m.page == page {
		color = cyan
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(color).Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
}

// helpLines describes contextual navigation and preserved global shortcuts.
func helpLines() []string {
	return []string{
		"AJUDA · NAVEGAÇÃO",
		"Tab / Shift+Tab  Próximo / painel anterior",
		"↑/↓ ou k/j       Selecionar item no painel",
		"←/→ ou h/l       Ajustar item selecionado",
		"Enter            Ativar ação selecionada",
		"",
		"ATALHOS GLOBAIS",
		"Espaço           Iniciar, pausar ou retomar timer",
		"1 / 2 / 3        Foco / pausa curta / pausa longa",
		"r                Reiniciar sessão atual",
		"- / +            Duração do modo (1–120 minutos)",
		"p                Tocar/parar rádio",
		"[ / ]            Estação anterior/próxima",
		", / .            Volume do rádio",
		"m                Iniciar/pausar mixer",
		"c / t            Aviso ligado/desligado / testar",
		"f                Modo foco",
		"Esc              Fechar ajuda ou sair do modo foco",
		"?                Abrir/fechar ajuda",
		"q / Ctrl+C       Sair",
		"",
		"Áudio e sessões não retomam ao reabrir.",
	}
}

// contextualHelp shows only operations that apply to the selected item.
func (m Model) contextualHelp() string {
	if m.width < 60 {
		if m.help {
			return "↑/↓ rolar · Esc voltar"
		}
		if m.page == 0 && (m.timerRow == 2 || m.timerRow == 3 || m.timerRow == 5) || m.page == 1 && m.radioRow == 2 || m.page == 2 && m.channel == 5 {
			return "↑↓ item · Enter ativar"
		}
		return "↑↓ item · ←→ ajustar · Enter"
	}
	if m.help {
		return "↑/↓ rolar · ?/Esc voltar"
	}
	if m.page == 0 {
		switch m.timerRow {
		case 0:
			return "↑/↓ selecionar · ←/→ modo · Enter próximo"
		case 1:
			return "↑/↓ selecionar · ←/→ minutos"
		case 4:
			return "↑/↓ selecionar · ←/→ ajustar · Enter alternar"
		default:
			return "↑/↓ selecionar · Enter ativar"
		}
	}
	if m.page == 1 {
		switch m.radioRow {
		case 0:
			return "↑/↓ selecionar · ←/→ estação · Enter próxima"
		case 1:
			return "↑/↓ selecionar · ←/→ volume"
		default:
			return "↑/↓ selecionar · Enter tocar/parar"
		}
	}
	if m.channel == 5 {
		return "↑/↓ selecionar · Enter iniciar/pausar"
	}
	return "↑/↓ canal · ←/→ volume · Enter iniciar/pausar"
}

// View keeps timer status and contextual shortcuts fixed around adaptive content.
func (m Model) View() string {
	if m.width < 30 || m.height < 10 {
		return fit("lofi & chill\n"+m.clockText()+" · "+m.timerState()+"\nAmplie para 30×10\nEspaço: timer · q: sair", m.width, m.height)
	}
	title := lipgloss.NewStyle().Foreground(pink).Bold(true).Render("▰ lofi & chill")
	if m.focus {
		title += " · MODO FOCO [Esc sair]"
	}
	status := fmt.Sprintf("%s %s · %s", [3]string{"Foco", "Curta", "Longa"}[m.timer.Mode], m.clockText(), m.timerState())
	if m.width >= 65 {
		status += fmt.Sprintf(" · %d focos hoje", m.timer.Completed)
	}
	nav := make([]string, 3)
	for i, name := range []string{"Timer", "Rádio", "Mixer"} {
		nav[i] = name
		if i == m.page {
			nav[i] = accent.Render("[> " + name + "]")
		}
	}
	bodyHeight := m.bodyHeight()
	var body string
	switch {
	case m.help:
		rows := helpLines()
		offset := min(m.helpOffset, max(0, len(rows)-bodyHeight))
		body = strings.Join(rows[offset:min(len(rows), offset+bodyHeight)], "\n")
	case !m.focus && m.width >= 96 && bodyHeight >= 19:
		left := (m.width - 2) / 2
		right := m.width - left - 2
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.box(0, left, bodyHeight), "  ", m.box(1, right, 8)+"\n\n"+m.box(2, right, bodyHeight-9))
	case bodyHeight >= 10:
		body = m.box(m.page, m.width, bodyHeight)
	default:
		body = m.panelContent(m.page, m.width, bodyHeight)
	}
	rows := strings.Split(fit(body, m.width, bodyHeight), "\n")
	for len(rows) < bodyHeight {
		rows = append(rows, "")
	}
	// Two short lines keep global actions discoverable without hiding panel hints.
	global := "Tab painel · espaço timer · ? ajuda · q sair"
	if m.width < 48 {
		global = "Tab · espaço timer · ? · q"
	}
	if m.width >= 70 {
		global += " · f foco"
	}
	if m.help {
		global = "Esc voltar · espaço timer · q"
	}
	footer := []string{muted.Render(strings.Repeat("─", m.width)), fit(m.notice, m.width, 1), m.contextualHelp(), global}
	return fit(title+"\n"+status+"\n"+strings.Join(nav, "  ")+"\n"+strings.Join(rows, "\n")+"\n"+strings.Join(footer, "\n"), m.width, m.height)
}

// fit clips terminal cells without damaging ANSI sequences.
func fit(value string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	rows := strings.Split(value, "\n")
	if len(rows) > height {
		rows = rows[:height]
	}
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], width, "…")
	}
	return strings.Join(rows, "\n")
}
