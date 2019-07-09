package main

// Test with: "kubectl run --generator=run-pod/v1 shell --rm -it --image ubuntu -- bash"
// See: https://github.com/kubernetes/kubernetes/blob/master/test/e2e/framework/exec_util.go

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

type Writer struct {
	Str []string
}

func (w *Writer) Write(p []byte) (n int, err error) {
	str := string(p)
	if len(str) > 0 {
		w.Str = append(w.Str, str)
	}
	return len(str), nil
}

func newStringReader(ss []string) io.Reader {
	formattedString := strings.Join(ss, "\n")
	reader := strings.NewReader(formattedString)
	return reader
}

func CheckKubeExec(req *Request) (string, interface{}) {
	config, err := clientcmd.BuildConfigFromFlags(req.masterURL, req.kubeconfigPath)
	if err != nil {
		return "UNKNOWN", err
	}
	kubeClient := kubernetes.NewForConfigOrDie(config)

	pod, err := kubeClient.CoreV1().Pods(req.Namespace).Get(req.Pod, metav1.GetOptions{})
	if err != nil {
		return "UNKNOWN", err
	}

	if req.Container != "" {
		notFound := true
		for _, container := range pod.Spec.Containers {
			if container.Name == req.Container {
				notFound = false
				break
			}
		}
		if notFound {
			return "UNKNOWN", fmt.Sprintf(`Container "%v" not found`, req.Container)
		}
	}

	log.Printf(`Container "%v" found`, req.Container)

	execRequest := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(req.Pod).
		Namespace(req.Namespace).
		SubResource("exec").
		Param("container", req.Container).
		Param("command", req.Command).
		Param("stdin", "true").
		Param("stdout", "false").
		Param("stderr", "false").
		Param("tty", "false")

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", execRequest.URL())
	if err != nil {
		return "UNKNOWN", err
	}

	stdIn := newStringReader([]string{"-c", req.Arg})
	stdOut := new(Writer)
	stdErr := new(Writer)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdIn,
		Stdout: stdOut,
		Stderr: stdErr,
		Tty:    false,
	})

	var exitCode int
	if err == nil {
		exitCode = 0
	} else {
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			exitCode = exitErr.ExitStatus()
		} else {
			return "UNKNOWN", fmt.Sprintf("Failed to find exit code: %v", err)
		}
	}

	output := fmt.Sprintf("Exit Code: %v", exitCode)
	if exitCode != 0 {
		exitCode = 2
	}

	return fmt.Sprintf("%v", exitCode), output
}

type Request struct {
	masterURL      string
	kubeconfigPath string

	Pod       string
	Container string
	Namespace string
	Command   string
	Arg       string
}

func NewCmd() *cobra.Command {
	var req Request
	c := &cobra.Command{
		Use:     "check_pod_exec",
		Short:   "Check exit code of exec command on Kubernetes container",
		Example: "",

		Run: func(cmd *cobra.Command, args []string) {
			req.Namespace = "default"
			req.Pod = "shell"
			fmt.Printf(CheckKubeExec(&req))
		},
	}

	c.Flags().StringVar(&req.masterURL, "master", req.masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	c.Flags().StringVar(&req.kubeconfigPath, "kubeconfig", req.kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	c.Flags().StringVarP(&req.Container, "container", "C", "", "Container name in specified pod")
	c.Flags().StringVarP(&req.Command, "cmd", "c", "/bin/sh", "Exec command. [Default: /bin/sh]")
	c.Flags().StringVarP(&req.Arg, "argv", "a", "", "Arguments for exec command. [Format: 'arg; arg; arg']")
	return c
}

func main() {
	c := NewCmd()
	if err := c.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
