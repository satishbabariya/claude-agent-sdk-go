package claudeagent

import "context"

// Query is the one-shot convenience form: spawn a Session, send a single
// prompt, close it, and return that turn's Result.
//
// Prefer NewSession + repeated Send for anything that sends more than one
// prompt to the same conversation (claude-mem's real observer does: an init
// prompt, then one per observation, then a summary) — each Query call pays a
// fresh spawn, with no context carried over from the last one.
func Query(ctx context.Context, prompt string, opts Options) (Result, error) {
	sess, err := NewSession(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	defer sess.Close()
	return sess.Send(prompt)
}
