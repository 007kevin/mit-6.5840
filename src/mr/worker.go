package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"time"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(
	mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	for run(mapf, reducef) {
		time.Sleep(time.Second)
	}
}

func run(
	mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) bool {
	resp := &GetTaskResponse{}
	ok := call("Coordinator.GetTask", &GetTaskRequest{}, resp)
	if !ok {
		fmt.Printf("call GetTask failed!\n")
		return true
	}

	if resp.Type == TERMINATE {
		return false
	}

	if resp.Type == WAIT {
		return true
	}

	if resp.Type == MAP {
		filename := resp.Key
		file, err := os.Open(resp.Key)
		if err != nil {
			fmt.Printf("cannot open %s: %s", filename, err.Error())
			return true
		}
		content, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Printf("cannot read %s: %s", filename, err.Error())
			return true
		}
		file.Close()
		kva := mapf(filename, string(content))
		fm := FileMapper{
			MapId: resp.Id,
			files: make(map[int]*os.File)}
		for _, kv := range(kva) {
			err := fm.WriteKv(ihash(kv.Key) % resp.NReduce, &kv)
			if err != nil {
				fmt.Printf("cannot encode: %s", err.Error())
				return true
			}
		}
		err = fm.Commit()
		if err != nil {
			fmt.Printf("cannot close file mapper: %s", err.Error())
			return true
		}
	} else {
		filepattern := fmt.Sprintf("mr-*-%d.json", resp.Id)
		filenames, err := filepath.Glob(filepattern)
		tmpname := fmt.Sprintf("mr-out-%d.%d", resp.Id, time.Now().Unix())
		finalname := fmt.Sprintf("mr-out-%d", resp.Id)
		if err != nil {
			fmt.Printf("cannot glob pattern %s: %s", filepattern, err.Error())
			return true
		}
		kvm := make(map[string][]string)
		for _, filename := range(filenames) {
			file, err := os.Open(filename)
			if err != nil {
				fmt.Printf("cannot read file %s: %s", filename, err.Error())
				return true
			}
			dec := json.NewDecoder(file)
			for {
				var kv KeyValue
				if err := dec.Decode(&kv); err != nil {
					break
				}
				kvm[kv.Key] = append(kvm[kv.Key], kv.Value)
			}
			err = file.Close()
			if err != nil {
				fmt.Printf("cannot close file %s: %s", filename, err.Error())
				return true
			}
		}
		keys := make([]string, 0, len(kvm))
		for key := range kvm {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		ofile, err := os.Create(tmpname)
		if err != nil {
			fmt.Printf("cannot create file %s: %s", tmpname, err.Error())
			return true
		}
		for _, key := range keys {
			output := reducef(key, kvm[key])
			_, err := fmt.Fprintf(ofile, "%s %s\n", key, output)
			if err != nil {
				fmt.Printf("cannot write value to file %s: %s", tmpname, err.Error())
			}
		}
		err = ofile.Close()
		err = os.Rename(tmpname, finalname)
		if err != nil {
			fmt.Printf("could not rename file %s to %s: %s", tmpname, finalname, err.Error())
			return true
		}
	}

	fmt.Printf("Received Type %d, Id %d\n", resp.Type, resp.Id)

	ok = call("Coordinator.CompleteTask", &CompleteTaskRequest{
		Id: resp.Id,
		Type: resp.Type,
	}, &CompleteTaskResponse{})
	if !ok {
		fmt.Printf("call CompleteTask failed!\n")
		return true
	}

	return true
}


//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
