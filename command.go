package mirvpgl

const (
	nullStr = byte('\x00')
)

func commandToByteSlice(cmd string) []byte {
	length := len("exec") + 1 + len(cmd) + 1 // "exec" + (nullStr) + command + (nullStr)
	command := make([]byte, 0, length)
	command = append(command, []byte("exec")...)
	command = append(command, nullStr)
	command = append(command, []byte(cmd)...)
	command = append(command, nullStr)
	return command
}
