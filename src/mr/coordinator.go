package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

const TIMEOUT = 10 * time.Second

// Order matters!
const (
	IDLE = iota
	IN_PROGRESS = iota
	DONE = iota
)

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b;
}

type Coordinator struct {
	MapTasks Tasks
	ReduceTasks Tasks
}

type Task struct {
	Id int
	Key string
	State int
	StartTime time.Time
	Lock sync.Mutex
}

type Tasks struct {
	tasks []*Task
}

// Returns the next IDLE tasks (if any). Idle tasks are returned as IN_PROGRESS
func (t *Tasks) GetWork() *Task {
	for _, task := range(t.tasks) {
		if task.State != DONE {
			task.Lock.Lock()
			if task.State == IDLE {
				task.State = IN_PROGRESS
				task.StartTime = time.Now()
				task.Lock.Unlock()
				return task
			}
			task.Lock.Unlock()
		}
	}
	return nil
}

func (t *Tasks) isDone() bool {
	for _, task := range(t.tasks) {
		if task.State != DONE {
			return false
		}
	}
	return true
}

// If a task runs longer than TIMEOUT, set it back to idle so another
// worker picks it up. Runs in the coordinator's Done() cycle
func (t *Tasks) EnforceTimeout() {
	for _, task := range(t.tasks) {
		if task.State != DONE {
			task.Lock.Lock()
			if task.State == IN_PROGRESS {
				elapsed := time.Now().Sub(task.StartTime);
				if elapsed >= TIMEOUT {
					task.State = IDLE
					task.StartTime = time.Time{};
				}
			}
			task.Lock.Unlock()
		}
	}
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

func (c *Coordinator) GetTask(req *GetTaskRequest, resp *GetTaskResponse) error {
	// Always return the reduce task count for ihash
	resp.NReduce = len(c.ReduceTasks.tasks)

	if !c.MapTasks.isDone() {
		task := c.MapTasks.GetWork()

		// All tasks are in progress
		if task == nil {
			resp.Type = WAIT
			return nil
		}

		// There is work to do
		resp.Type = MAP
		resp.Id = task.Id
		resp.Key = task.Key
		return nil
	}

	if !c.ReduceTasks.isDone() {
		task := c.ReduceTasks.GetWork()

		// All tasks are in progress
		if task == nil {
			resp.Type = WAIT
			return nil
		}

		// There is work to do
		resp.Type = REDUCE
		resp.Id = task.Id
		resp.Key = task.Key
		return nil
	}

	// Terminate
	resp.Type = TERMINATE
	return nil
}

func (c *Coordinator) CompleteTask(req *CompleteTaskRequest, resp *CompleteTaskResponse) error {
	task, err := c.getTaskByIndex(req.Type, req.Id)
	if err != nil {
		return err
	}
	task.Lock.Lock()
	defer task.Lock.Unlock()
	task.State = DONE
	return nil
}

func (c *Coordinator) getTaskByIndex(taskType int, index int) (*Task, error) {
	switch taskType {
	case MAP:
		if index >= len(c.MapTasks.tasks) {
			return nil, fmt.Errorf("CompleteTask: map task index %d is out of bounds", index)
		}
		return c.MapTasks.tasks[index], nil
	case REDUCE:
		if index >= len(c.ReduceTasks.tasks) {
			return nil, fmt.Errorf("CompleteTask: reduce task index %d is out of bounds", index)
		}
		return c.ReduceTasks.tasks[index], nil
	default:
		return nil, fmt.Errorf("CompleteTask: unrecognized type %d", taskType)
	}
}



//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.
	c.MapTasks.EnforceTimeout()
	c.ReduceTasks.EnforceTimeout()

	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		MapTasks: Tasks{ mapTasks(files) },
		ReduceTasks: Tasks{ reduceTasks(nReduce) },
	}

	c.server()
	return &c
}

func mapTasks(files []string) []*Task {
	tasks := []*Task{}
	for i, file := range(files){
		tasks = append(tasks, &Task{
			Id: i,
			State: IDLE,
			Key: file,
			StartTime: time.Time{},
		})
	}
	return tasks
}

func reduceTasks(nReduce int) []*Task {
	tasks := []*Task{}
	for i := 0; i < nReduce; i++ {
		tasks = append(tasks, &Task{
			Id: i,
			State: IDLE,
			Key: "",
			StartTime: time.Time{},
		})

	}
	return tasks
}
