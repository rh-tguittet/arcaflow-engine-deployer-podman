package cli_wrapper

import (
	"arcaflow-engine-deployer-podman/util"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type cliWrapper struct {
	PodmanFullPath string
	stdoutBuffer   bytes.Buffer
}

func NewCliWrapper(fullPath string) CliWrapper {
	return &cliWrapper{
		PodmanFullPath: fullPath,
	}
}

func (p *cliWrapper) decorateImageName(image string) string {
	imageParts := strings.Split(image, ":")
	if len(imageParts) == 1 {
		image = fmt.Sprintf("%s:latest", image)
	}
	return image
}

func (p *cliWrapper) commandSetEnv(command *[]string, env []string) {
	for _, v := range env {
		if tokens := strings.Split(v, "="); len(tokens) == 2 {
			*command = append(*command, "-e", v)
		}
	}
}

func (p *cliWrapper) commandSetVolumes(command *[]string, binds []string) {
	for _, v := range binds {
		if tokens := strings.Split(v, ":"); len(tokens) == 2 {
			*command = append(*command, "-v", v)
		}
	}
}

func (p *cliWrapper) commandSetCgroupNs(command *[]string, cgroupNs string) {
	if cgroupNs != "" {
		*command = append(*command, "--cgroupns", cgroupNs)
	}
}

func (p *cliWrapper) commandSetContainerName(command *[]string, name string) {
	if name != "" {
		*command = append(*command, "--name", name)
	}
}

func (p *cliWrapper) ImageExists(image string) (*bool, error) {
	image = p.decorateImageName(image)
	cmd := exec.Command(p.PodmanFullPath, "image", "ls", "--format", "{{.Repository}}:{{.Tag}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	outStr := out.String()
	outSlice := strings.Split(outStr, "\n")
	exists := util.SliceContains(outSlice, image)
	return &exists, nil
}

func (p *cliWrapper) PullImage(image string, platform *string) error {
	commandArgs := []string{"pull"}
	if platform != nil {
		commandArgs = append(commandArgs, []string{"--platform", *platform}...)
	}
	image = p.decorateImageName(image)
	commandArgs = append(commandArgs, image)
	cmd := exec.Command(p.PodmanFullPath, commandArgs...)
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return errors.New(out.String())
	}
	return nil
}

func (p *cliWrapper) Deploy(image string, containerName string, args []string) (io.WriteCloser, io.ReadCloser, io.ReadCloser, *exec.Cmd, error) {
	image = p.decorateImageName(image)
	args = append(args, image)
	cmd := exec.Command(p.PodmanFullPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, errors.New(err.Error())
	}
	time.Sleep(5 * time.Second)
	go p.readStdout(stdout)
	return stdin, stdout, stderr, cmd, nil
}

func (p *cliWrapper) _readStdout(stdout io.ReadCloser) {
	reader := bufio.NewScanner(stdout)
	for reader.Scan() {
		p.stdoutBuffer.Write(reader.Bytes())
	}

}
func (p *cliWrapper) readStdout(stdout io.ReadCloser) {
	writer := bufio.NewWriter(&p.stdoutBuffer)
	var out []byte
	buf := make([]byte, 1024, 1024)
	for {
		n, err := stdout.Read(buf[:])
		if n > 0 {
			d := buf[:n]
			out = append(out, d...)
			_, err := writer.Write(d)
			if err != nil {
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
	}
}

func (p *cliWrapper) GetStdoutData() []byte {
	bufBytes := p.stdoutBuffer.Bytes()
	return bufBytes
}

func (p *cliWrapper) ClearBuffer() {
	p.stdoutBuffer.Reset()
}