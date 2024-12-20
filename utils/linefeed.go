package utils

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/moodclient/telnet"
)

type LineFeedConfig struct {
	MaxLength         int
	CharacterMode     bool
	SuppressLocalEcho bool
}

type LineFeed struct {
	terminal *telnet.Terminal
	parser   *telnet.TerminalDataParser

	LineOut telnet.TerminalDataHandler
	EchoOut telnet.TerminalDataHandler

	lineLock sync.Mutex

	justPushedCR bool

	config LineFeedConfig

	cursorPos      int
	currentLine    []rune
	visibleIndices []int
}

func NewLineFeed(terminal *telnet.Terminal, lineOut, echoOut telnet.TerminalDataHandler, config LineFeedConfig) *LineFeed {
	return &LineFeed{
		terminal: terminal,
		parser:   telnet.NewTerminalDataParser(),

		LineOut: lineOut,
		EchoOut: echoOut,

		config: config,
	}
}

func (l *LineFeed) insertData(newRunes string, visible bool) {
	if l.config.MaxLength > 0 && len(l.visibleIndices) >= l.config.MaxLength {
		l.echo(telnet.TextData(string(ansi.BEL)))
		return
	} else if l.config.MaxLength > 0 && visible && len(l.visibleIndices)+len(newRunes) > l.config.MaxLength {
		remainingLength := l.config.MaxLength - len(l.visibleIndices)
		newRunes = newRunes[:remainingLength]
		l.echo(telnet.TextData(string(ansi.BEL)))
	}

	// We build a line using 3 components:
	// 1. the current line including both visible and invisible text.
	// 2. A list of indexes for every rune that is visible
	// 3. The location of the cursor, which is an index in the list of visible indices
	//
	// That's right, the cursorPos is an index of an index. Very confusing.
	//
	// In order to insert data we must do the following:
	//
	// 1. The new runes must be inserted into the current line
	// 2. If we are adding text the middle of the current line, all visible indices
	//    after the cursor location must be adjusted upwards to account for the
	//    new runes
	// 3. If the new text is visible, we must insert the new visible indices
	// 4. If the new text is visible, we must advance the cursor position

	// Step 1 - insert the runes and also get a rune count while we're at it
	runeCount := 0
	cursorLocation := len(l.currentLine)
	if l.cursorPos < len(l.visibleIndices) {
		cursorLocation = l.visibleIndices[l.cursorPos]
	}

	for idx, r := range newRunes {
		runeCount++
		l.currentLine = slices.Insert(l.currentLine, cursorLocation+idx, r)
	}

	// Step 2 - adjust indices
	for i := l.cursorPos; i < len(l.visibleIndices); i++ {
		l.visibleIndices[i] += runeCount
	}

	if visible {
		// Step 3 - insert new visible indices
		// Step 4 - advance cursor position
		for i := 0; i < runeCount; i++ {
			l.visibleIndices = slices.Insert(l.visibleIndices, l.cursorPos, cursorLocation)
			l.cursorPos++
			cursorLocation++
		}
	}

	if !visible {
		return
	}

	// Echoing new text:
	// 1. if the cursor is at the end, just write the new text
	if l.cursorPos >= len(l.visibleIndices) {
		l.echo(telnet.TextData(newRunes))
		return
	}

	var update strings.Builder
	// 2. Otherwise, clear the rest of the line
	update.WriteString("\x1b[K")

	// - Rewrite the line from the space before the cursor
	textPos := l.visibleIndices[l.cursorPos-1]
	for i := textPos; i < len(l.currentLine); i++ {
		update.WriteRune(l.currentLine[i])
	}

	// - Reset the cursor position
	writtenSpaces := len(l.visibleIndices) - l.cursorPos
	update.WriteRune('\x1b')
	update.WriteRune('[')
	update.WriteString(strconv.Itoa(writtenSpaces))
	update.WriteRune('D')

	l.echo(telnet.TextData(update.String()))
}

func (l *LineFeed) moveCursor(delta int) bool {
	startPos := l.cursorPos
	l.cursorPos += delta
	if l.cursorPos < 0 {
		l.cursorPos = 0
	} else if l.cursorPos > len(l.visibleIndices) {
		l.cursorPos = len(l.visibleIndices)
	}

	realDelta := l.cursorPos - startPos

	if realDelta > 0 {
		l.echo(telnet.TextData(fmt.Sprintf("\x1b[%dC", realDelta)))
	} else if realDelta < 0 {
		l.echo(telnet.TextData(fmt.Sprintf("\x1b[%dD", -realDelta)))
	}

	return realDelta != 0
}

func (l *LineFeed) deleteAtCursor() {
	if l.cursorPos >= len(l.visibleIndices) {
		return
	}

	// Dealing with a delete:
	//
	// 1. Remove excess characters from rune slice
	// 2. Remove excess visible indices from visible indices slice
	// 3. Adjust visible indices after cursor to account for missing runes
	// 4. Fix echoed line

	cursorTextPos := l.visibleIndices[l.cursorPos]
	nextCursorPos := l.cursorPos + 1
	nextTextPos := len(l.currentLine)

	if nextCursorPos < len(l.visibleIndices) {
		nextTextPos = l.visibleIndices[nextCursorPos]
	}

	l.currentLine = slices.Delete(l.currentLine, cursorTextPos, nextTextPos)
	l.visibleIndices = slices.Delete(l.visibleIndices, l.cursorPos, nextCursorPos)

	for i := l.cursorPos; i < len(l.visibleIndices); i++ {
		l.visibleIndices[i] -= (nextTextPos - cursorTextPos)
	}

	// Fixing the echoed line:
	//
	// 1. Clear all text from the cursor position
	// 2. Rewrite all text after cursor
	// 3. Walk the cursor back by number of positions written
	var echo strings.Builder
	// step 1
	echo.WriteString("\x1b[K")

	// step 2
	for i := cursorTextPos; i < len(l.currentLine); i++ {
		echo.WriteRune(l.currentLine[i])
	}

	visiblePositions := len(l.visibleIndices) - l.cursorPos
	if visiblePositions > 0 {
		echo.WriteRune('\x1b')
		echo.WriteRune('[')
		echo.WriteString(strconv.Itoa(visiblePositions))
		echo.WriteRune('D')
	}

	l.echo(telnet.TextData(echo.String()))
}

func (l *LineFeed) Flush(newline bool) {
	if len(l.currentLine) == 0 {
		return
	}

	l.lineLock.Lock()
	defer l.lineLock.Unlock()

	l.flush(newline)
}

func (l *LineFeed) echo(data telnet.TerminalData) {
	if !l.config.CharacterMode && !l.config.SuppressLocalEcho {
		l.EchoOut(l.terminal, data)
	}
}

func (l *LineFeed) flush(newline bool) {
	if len(l.currentLine) == 0 {
		return
	}

	if newline {
		l.currentLine = append(l.currentLine, '\r', '\n')
	}

	l.parser.FireSingle(l.terminal, string(l.currentLine), l.LineOut)

	l.cursorPos = 0
	l.currentLine = l.currentLine[:0]
	l.visibleIndices = l.visibleIndices[:0]
}

func (l *LineFeed) controlCodeIn(sequence telnet.ControlCodeData) {
	switch sequence {
	case '\r':
		l.justPushedCR = true
		l.echo(telnet.ControlCodeData(ansi.ControlCode('\r')))
		l.echo(telnet.ControlCodeData(ansi.ControlCode('\n')))
		l.flush(true)
	case '\n':
		if !l.justPushedCR {
			l.echo(telnet.ControlCodeData(ansi.ControlCode('\r')))
			l.echo(telnet.ControlCodeData(ansi.ControlCode('\n')))
			l.flush(true)
		}
	case ansi.DEL, ansi.BS:
		if l.moveCursor(-1) {
			l.deleteAtCursor()
		}
	}
}

func (l *LineFeed) csiSequenceIn(sequence telnet.CsiData) {
	switch sequence.Cmd.Command() {
	case 'C':
		// Cursor forward
		delta, _ := sequence.Param(0, 1)
		l.moveCursor(delta)
		return
	case 'D':
		// Cusror backward
		delta, _ := sequence.Param(0, 1)
		l.moveCursor(-delta)
		return
	case '~':
		param, hasParam := sequence.Param(0, -1)
		if hasParam && param == 3 {
			// Delete ESC[3~
			l.deleteAtCursor()
			return
		}
	}
	l.insertData(sequence.String(), false)
}

func (l *LineFeed) LineIn(t *telnet.Terminal, data telnet.TerminalData) {
	l.lineLock.Lock()
	defer l.lineLock.Unlock()

	if l.config.CharacterMode {
		l.LineOut(t, data)

		controlCode, isControlCode := data.(telnet.ControlCodeData)
		if isControlCode && controlCode == ansi.CR {
			l.LineOut(t, telnet.ControlCodeData(ansi.LF))
		}

		return
	}

	hadPushedCR := l.justPushedCR

	switch d := data.(type) {
	case telnet.TextData:
		l.insertData(d.String(), true)
	case telnet.ControlCodeData:
		l.controlCodeIn(d)
	case telnet.CsiData:
		l.csiSequenceIn(d)
	default:
		l.insertData(d.String(), false)
	}

	if hadPushedCR {
		l.justPushedCR = false
	}
}

func (l *LineFeed) CharacterMode() bool {
	l.lineLock.Lock()
	defer l.lineLock.Unlock()

	return l.config.CharacterMode
}

func (l *LineFeed) SetCharacterMode(charMode bool) {
	l.lineLock.Lock()
	defer l.lineLock.Unlock()

	l.config.CharacterMode = charMode
}

func (l *LineFeed) SuppressLocalEcho() bool {
	l.lineLock.Lock()
	defer l.lineLock.Unlock()

	return l.config.SuppressLocalEcho
}

func (l *LineFeed) SetSuppressLocalEcho(suppress bool) {
	l.lineLock.Lock()
	defer l.lineLock.Unlock()

	l.config.SuppressLocalEcho = suppress
}

func (l *LineFeed) Text() string {
	var sb strings.Builder

	for _, visibleIndex := range l.visibleIndices {
		sb.WriteRune(l.currentLine[visibleIndex])
	}

	return sb.String()
}
