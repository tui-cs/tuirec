package keystroke

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

// Sleeper pauses between played actions.
type Sleeper interface {
	Sleep(time.Duration)
}

type realSleeper struct{}

// Player writes parsed keystroke actions to an output stream.
type Player struct {
	writer         io.Writer
	sleeper        Sleeper
	keystrokeDelay time.Duration
	kittyKeyboard  bool
	beforeAction   func(Action)
	log            io.Writer
}

// PlayerOption configures a Player.
type PlayerOption func(*Player)

// WithLogWriter logs played actions and pacing to writer.
func WithLogWriter(writer io.Writer) PlayerOption {
	return func(player *Player) {
		player.log = writer
	}
}

// WithKittyKeyboard enables Kitty keyboard protocol encoding.
// When enabled, all keystrokes are emitted as CSI u sequences
// providing full modifier disambiguation.
func WithKittyKeyboard() PlayerOption {
	return func(player *Player) {
		player.kittyKeyboard = true
	}
}

// WithBeforeAction registers a callback invoked before each action is played.
func WithBeforeAction(fn func(Action)) PlayerOption {
	return func(player *Player) {
		player.beforeAction = fn
	}
}

// NewPlayer creates a keystroke player.
// writer must not be nil; passing nil will cause Play/PlayActions to return an error.
func NewPlayer(writer io.Writer, sleeper Sleeper, keystrokeDelay time.Duration, options ...PlayerOption) Player {
	if sleeper == nil {
		sleeper = realSleeper{}
	}

	player := Player{
		writer:         writer,
		sleeper:        sleeper,
		keystrokeDelay: keystrokeDelay,
	}
	for _, option := range options {
		option(&player)
	}

	return player
}

// Play parses and plays a keystroke script.
func (p Player) Play(script string) error {
	actions, err := Parse(script)
	if err != nil {
		return err
	}

	return p.PlayActions(actions)
}

// PlayActions plays pre-parsed actions.
func (p Player) PlayActions(actions []Action) error {
	for _, action := range actions {
		if err := p.playAction(action); err != nil {
			return err
		}
	}

	return nil
}

func (p Player) playAction(action Action) error {
	if p.beforeAction != nil {
		p.beforeAction(action)
	}
	switch action.Kind {
	case Wait:
		p.logf("wait %s (%s)\n", actionName(action), action.Delay)
		p.sleeper.Sleep(action.Delay)
	case Write:
		sequence := action.Sequence
		if p.kittyKeyboard {
			if resolved, ok := resolveKittyKey(action.Label); ok {
				sequence = resolved
			}
		}
		p.logf("key %s -> %s; delay %s\n", actionName(action), strconv.Quote(sequence), p.keystrokeDelay)
		if err := p.write(sequence); err != nil {
			return err
		}
		p.sleeper.Sleep(p.keystrokeDelay)
	case Literal:
		return p.writeLiteral(action.Sequence)
	default:
		return fmt.Errorf("unknown action kind: %d", action.Kind)
	}

	return nil
}

func (p Player) writeLiteral(value string) error {
	first := true
	for _, r := range value {
		if !first {
			p.logf("literal delay %s\n", p.keystrokeDelay)
			p.sleeper.Sleep(p.keystrokeDelay)
		}
		first = false

		char := string(r)
		if p.kittyKeyboard {
			char = kittyCsiU(int(r), 0)
		}
		p.logf("literal %s\n", strconv.QuoteRune(r))
		if err := p.write(char); err != nil {
			return err
		}
	}

	return nil
}

func (p Player) logf(format string, args ...any) {
	if p.log == nil {
		return
	}

	fmt.Fprintf(p.log, "tuirec: "+format, args...)
}

func actionName(action Action) string {
	if action.Label != "" {
		return action.Label
	}

	if action.Sequence != "" {
		return strconv.Quote(action.Sequence)
	}

	return action.Kind.String()
}

// String returns a stable label for a Kind.
func (kind Kind) String() string {
	switch kind {
	case Wait:
		return "wait"
	case Write:
		return "write"
	case Literal:
		return "literal"
	default:
		return fmt.Sprintf("kind(%d)", kind)
	}
}

func (p Player) write(value string) error {
	if p.writer == nil {
		return fmt.Errorf("write keystroke: nil writer")
	}

	_, err := io.WriteString(p.writer, value)
	if err != nil {
		return fmt.Errorf("write keystroke: %w", err)
	}

	return nil
}

func (realSleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}
