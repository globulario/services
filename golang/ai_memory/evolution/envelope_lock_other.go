//go:build !unix

package evolution

// withEnvelopeLock has no advisory-locking implementation on this platform.
// Running concurrent scenario invocations against one ChangeEnvelope is not
// supported here; a single invocation behaves identically to the locked path.
func withEnvelopeLock(_ string, fn func() error) error {
	return fn()
}
