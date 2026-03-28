# termtest/xpty

Xpty provides an abstraction to run a terminal application in a pseudo-terminal environment for Linux, Mac and Windows. On Windows it uses the [ActiveState/termtest/conpty](https://github.com/ActiveState/termtest/conpty) to run the application inside of a [ConPTY terminal](https://devblogs.microsoft.com/commandline/windows-command-line-introducing-the-windows-pseudo-console-conpty/). The pseudo-terminal is automatically attached to a virtual terminal that is compatible with an `xterm`-terminal.

## The problem

Attaching the pseudo-terminal to an `xterm`-compatible virtual terminal is for the following reason:

If the terminal application sends  a cursor position request (CPR) signal, the application usually blocks on read until it receives the response (the column and row number of the cursor) from terminal. `xpty` helps unblocking such programmes, as it actually generates the awaited response.

## Rune-by-rune streaming

Reading from the underlying terminal is done with the `ReadRune()` function that returns the next interpretable rune. Such fine-grained and slow output processing allows us to keep the state of the virtual terminal deterministic.

## Example

```go
xp, _ := xpty.New(20, 10)
defer xp.Close()

cmd := exec.Command("/bin/bash")
xp.StartProcessInTerminal(cmd)

xp.TerminalInPipe().WriteString("echo hello world\n")
xp.TerminalInPipe().WriteString("exit\n")

buf := new(bytes.Buffer)
n, _ := xp.WriteTo(buf)

fmt.Printf("Raw output:\n%s\n", buf.String())
fmt.Printf("Terminal output:\n%s\n", xp.State.String())
```
