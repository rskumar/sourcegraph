package sgx

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"src.sourcegraph.com/sourcegraph/pkg/gitserver"
	"src.sourcegraph.com/sourcegraph/sgx/cli"
)

func init() {
	_, err := cli.CLI.AddCommand("git-server",
		"run git server",
		"A server to run git commands remotely.",
		&gitServerCmd{},
	)
	if err != nil {
		log.Fatal(err)
	}
}

type gitServerCmd struct {
}

func (c *gitServerCmd) Execute(args []string) error {
	go func() {
		io.Copy(ioutil.Discard, os.Stdin)
		log.Fatal("git-server: stdin closed, terminating")
	}()

	gitserver.RegisterHandler()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	fmt.Printf("git-server: listening on %s\n", l.Addr())
	return http.Serve(l, nil)
}