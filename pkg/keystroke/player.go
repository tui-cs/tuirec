package keystroke

import (
	"fmt"
	"io"
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
}

// NewPlayer creates a keystroke player.
func NewPlayer(writer io.Writer, sleeper Sleeper, keystrokeDelay time.Duration) Player {
	if sleeper == nil {
		sleeper = realSleeper{}
	}

	return Player{
		writer:         writer,
		sleeper:        sleeper,
		keystrokeDelay: keystrokeDelay,
	}
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
	switch action.Kind {
	case Wait:
		p.sleeper.Sleep(action.Delay)
	case Write:
		if err := p.write(action.Sequence); err != nil {
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
			p.sleeper.Sleep(p.keystrokeDelay)
		}
		first = false

		if err := p.write(string(r)); err != nil {
			return err
		}
	}

	return nil
}

func (p Player) write(value string) error {
	_, err := io.WriteString(p.writer, value)
	if err != nil {
		return fmt.Errorf("write keystroke: %w", err)
	}

	return nil
}

func (realSleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}
