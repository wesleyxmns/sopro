package updater

import (
	"fmt"
	"os/exec"
	"runtime"
)

// ElevatedCommand prepara uma nova execução do Sopro por sudo sem envolver um shell.
// O chamador continua responsável por conectar stdin/stdout e executar o comando.
func ElevatedCommand(executable string, args ...string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("%w: elevação automática indisponível no Windows", ErrPermissionDenied)
	}
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return nil, fmt.Errorf("%w: o comando sudo não está disponível", ErrPermissionDenied)
	}
	return exec.Command(sudoPath, append([]string{executable}, args...)...), nil
}
