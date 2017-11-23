package daemon

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	CMD_FAILED    = 1
	DAEMON_FAILED = 2
	PARENT_FAILED = 3
)

var isDaemon bool
var isParent bool
var isChild bool
var op string
var serving bool

var cmdpath string
var restarted bool

func init() {
	var err error
	cmdpath, err = filepath.Abs(os.Args[0])
	if err != nil {
		os.Stderr.WriteString("failed to get command path, err:" + err.Error())
		return
	}
	parse_cmds()
	if !isDaemon {
		if op == "" {
			return
		} else {
			operate(op)
		}
	}

	if isChild {
		return
	}

	if !isParent {
		daemon()
	} else {
		parent()
	}
}

func parent() {
	childexit, child := create_child()
	closesig, stop, restart := create_signal()
	serving = true
	for serving {
		select {
		case _ = <-stop:
			if err := child.Signal(os.Kill); err == nil {
				<-childexit
			}

		case _ = <-restart:
			if err := child.Signal(os.Kill); err == nil {
				<-childexit
			}
			childexit, child = create_child()

		case _ = <-childexit:
			if serving {
				childexit, child = create_child()
			}
		}
	}
	<-closesig
	os.Exit(0)
}

func parse_cmds() {
	for i, arg := range os.Args {
		if arg == "--__CHILD__" {
			isChild = true
		}
		if arg == "--__PARENT__" {
			isParent = true
		}
		if arg == "--daemon" || arg == "-daemon" {
			isDaemon = true
		}
		if arg == "--signal" || arg == "-signal" {
			if len(os.Args) > i+1 {
				op = os.Args[i+1]
			}
		}
	}
}

func create_child() (chan int, *os.Process) {
	childexit := make(chan int, 1)
	if !restarted {
		os.Args = append(os.Args, "--__CHILD__")
		restarted = true
	}
	cmd := exec.Command(cmdpath, os.Args[1:]...)
	if err := cmd.Start(); err != nil {
		os.Stderr.WriteString("failed to start child, err:" + err.Error())
		childexit <- 1
		return childexit, cmd.Process
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			os.Stderr.WriteString("failed to wait child, err:" + err.Error())
		}
		childexit <- 1
	}()
	return childexit, cmd.Process
}

func create_signal() (chan int, chan int, chan int) {
	procdir := filepath.Dir(cmdpath) + "/.proc"
	sockpath := procdir + "/sock"
	ln, err := net.Listen("unix", sockpath)
	if err != nil {
		exit(err, "failed to listen sock:"+sockpath, PARENT_FAILED)
	}
	closesign := make(chan int, 0)
	stop := make(chan int, 0)
	restart := make(chan int, 0)
	go func() {
		for serving {
			conn, err := ln.Accept()
			if err != nil {
				os.Stderr.WriteString("failed to accept command:" + err.Error())
				continue
			}
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				operation := scanner.Text()
				switch operation {
				case "stop":
					serving = false
					stop <- 1

				case "restart":
					restart <- 1
				}
			}
			conn.Close()
		}
		ln.Close()
		closesign <- 1
	}()
	return closesign, stop, restart
}

func daemon() {
	procdir := filepath.Dir(cmdpath) + "/.proc"
	sockpath := procdir + "/sock"
	if _, err := os.Stat(sockpath); !os.IsNotExist(err) {
		exit(nil, "daemon already exists, see sock:"+sockpath, DAEMON_FAILED)
	}
	if _, err := os.Stat(procdir); os.IsNotExist(err) {
		if err := os.Mkdir(procdir, 0700); err != nil {
			exit(err, "failed to create procdir:"+procdir, DAEMON_FAILED)
		}
	}
	os.Args = append(os.Args, "--__PARENT__")
	cmd := exec.Command(cmdpath, os.Args[1:]...)
	if err := cmd.Start(); err != nil {
		exit(err, "failed to start command", DAEMON_FAILED)
	}

	// no waiting for command process
	os.Exit(0)
}

func exit(err error, msg string, code int) {
	if err != nil {
		msg += ",err:" + err.Error()
	}
	os.Stderr.WriteString(msg)
	os.Exit(code)
}

func operate(operation string) {
	procdir := filepath.Dir(cmdpath) + "/.proc"
	sockpath := procdir + "/sock"
	conn, err := net.Dial("unix", sockpath)
	if err != nil {
		exit(err, "failed to connect to daemon", CMD_FAILED)
	}
	switch operation {
	case "restart":
	case "stop":
	default:
		exit(nil, "wrong operation:"+operation, CMD_FAILED)
	}
	_, err = conn.Write([]byte(operation + "\n"))
	if err != nil {
		exit(err, "failed to send command:"+operation, CMD_FAILED)
	}
	conn.Close()
	os.Exit(0)
}
