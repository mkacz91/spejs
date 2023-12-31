package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mkacz91/spejs/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Job struct {
	name     string
	state    JobState
	cmd      *exec.Cmd
	port     int
	conn     *grpc.ClientConn
	service  pb.JobServiceClient
	cmdEnded chan bool
}

type JobState int

const (
	NotStarted JobState = iota
	Starting
	Running
	Ready
	Stopping
	Stopped
	Abandoned
)

const (
	greenOK = "\033[32mOK\033[0m"
	redFAIL = "\033[31mFAIL\033[0m"
)

func NewJob(path string, portKey string, port int, arg ...string) *Job {
	arg = append(arg, fmt.Sprintf("%s=%d", portKey, port))
	job := Job{
		name: filepath.Base(path),
		cmd:  exec.Command(path, arg...),
		port: port,
	}
	return &job
}

func (job *Job) State() JobState {
	return job.state
}

func (job *Job) Start(timeout time.Duration) error {
	if job.state != NotStarted {
		return fmt.Errorf("Job already started")
	}
	job.state = Starting
	success := false
	defer func() {
		if success {
			job.state = Running
		} else {
			job.Stop(timeout)
		}
	}()

	tmpout, err := os.CreateTemp("", "spejs_"+job.name+"_OUT_")
	if err != nil {
		fmt.Println("Unable to create temporary output file: ", err)
		return err
	}
	job.cmd.Stdout = tmpout
	job.cmd.Stderr = tmpout

	fmt.Println("Command:")
	fmt.Println("  ", job.cmd)
	fmt.Print("Starting ... ")
	err = job.cmd.Start()
	if err != nil {
		fmt.Println(redFAIL)
		fmt.Println(err)
		return err
	}
	fmt.Println(greenOK)
	fmt.Println("Output file:", tmpout.Name())

	job.cmdEnded = make(chan bool)
	go func() {
		job.cmd.Wait()
		job.cmdEnded <- true
	}()

	dialTarget := fmt.Sprintf("localhost:%d", job.port)
	fmt.Print("Dialing gRPC endpoint: ", dialTarget, " ... ")
	job.conn, err = grpc.Dial(
		dialTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Println(redFAIL)
		fmt.Println(err)
		return err
	}
	fmt.Println(greenOK)

	job.service = pb.NewJobServiceClient(job.conn)

	statusCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var lastStatusErr error
	fmt.Print("Querying status of ", job.name, " ")
	for {
		time.Sleep(1 * time.Second)
		fmt.Print(".")
		response, err := job.service.Status(statusCtx, &pb.Empty{})
		if err == nil && !response.IsReady {
			err = fmt.Errorf("Job responded, saying it's not ready")
		}
		if err != nil {
			if statusCtx.Err() != nil {
				fmt.Println("", redFAIL)
				// The context has timed out and the current error tells simply that.
				// The previous one, if present, is more helpful.
				if lastStatusErr != nil {
					err = lastStatusErr
				}
				fmt.Println(err)
				return err
			} else {
				lastStatusErr = err
			}
		} else {
			fmt.Println("", greenOK)
			break
		}
	}

	success = true
	return nil
}

func (job *Job) Stop(timeout time.Duration) error {
	if job.state != Starting && job.state != Running && job.state != Ready {
		return fmt.Errorf("job not started")
	}
	if job.state == Starting {
		job.state = Stopped
		return nil
	}
	job.state = Stopping

	success := false

	if job.service != nil {
		fmt.Print("Gracefully quitting ", job.name, " using its JobService.Quit RPC ... ")
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_, err := job.service.Quit(ctx, &pb.Empty{})
		if err != nil {
			fmt.Println(redFAIL)
			fmt.Println(err)
		} else {
			fmt.Println(greenOK)
			fmt.Print("Waiting for the process to terminate ... ")
			select {
			case <-job.cmdEnded:
				fmt.Println(greenOK)
				success = true
			case <-time.After(timeout):
				fmt.Println(redFAIL)
				fmt.Println("Still running after", timeout)
			}
		}
	}

	if !success {
		fmt.Print("Forcefully quitting ", job.name, " ... ")
		job.cmd.Process.Signal(os.Kill)
		select {
		case <-job.cmdEnded:
			fmt.Println(greenOK)
			success = true
		case <-time.After(timeout):
			fmt.Println(redFAIL)
			fmt.Println("Still running after", timeout, ". PID=", job.cmd.Process.Pid)
		}
	}

	if job.conn != nil {
		job.conn.Close()
	}

	if success {
		job.state = Stopped
		fmt.Println(
			"Job", job.name, "exited with", job.cmd.ProcessState.ExitCode())
		return nil
	} else {
		job.state = Abandoned
		err := fmt.Errorf("Job %s abandoned", job.name)
		fmt.Println(err)
		return err
	}
}
