package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

const (
	LEADER = "LEADER"
	FOLLOWER = "FOLLOWER"
	CANDIDATE = "CANDIDATE"
)

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	is string       // Current state
	heartbeat int32   // Increment with AppendEntries, decrement with ticker
	currentTerm int
	votedFor int


}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// Your code here (2A).
	term := rf.currentTerm
	isleader := rf.get() == LEADER
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}


// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}


// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}


// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	Term int
	CandidateId int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	Term int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		fmt.Println("DEBUG got request: " + rf.string() + " -> no vote (1)")
		return
	}

	if args.Term > rf.currentTerm {
		rf.mu.Lock()
		rf.set(FOLLOWER)
		rf.setTerm(args.Term)
		rf.votedFor = args.CandidateId
		reply.Term = rf.currentTerm
		reply.VoteGranted = true
		 // reset election timer if vote granted
		atomic.StoreInt32(&rf.heartbeat, 1);
		rf.mu.Unlock()
		fmt.Println("DEBUG got request: " + rf.string() + " -> yes vote (1)")
		return
	}

	if rf.votedFor == -1 {
		rf.votedFor = args.CandidateId
	}
	if rf.votedFor == args.CandidateId {
		reply.Term = rf.currentTerm
		reply.VoteGranted = true
		 // reset election timer if vote granted
		atomic.StoreInt32(&rf.heartbeat, 1);
		fmt.Println("DEBUG got request: " + rf.string() + " -> yes vote (2)")
		return
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false
	fmt.Println("DEBUG got request: " + rf.string() + " -> no vote (2)")
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
func (rf *Raft) requestVotes(args *RequestVoteArgs, reply *RequestVoteReply) {
	ch := make(chan *RequestVoteReply)
	for i := range(rf.peers) {
		if (i == rf.me) {
			continue; // make sure to skip itself to avoid deadlock
		}
		go func(i int) {
			var r RequestVoteReply
			if rf.sendRequestVote(i, args, &r) {
				ch <- &r
			} else {
				ch <- &RequestVoteReply{VoteGranted: false, Term: -1}
			}
		}(i)
	}
	voted := 1 // count current candidate's vote
	for i := 0; i < len(rf.peers); i++ {
		r := <-ch
		if (r.Term < args.Term){
			continue;
		}
		if (r.Term > args.Term) {
			reply.Term = r.Term
			reply.VoteGranted = r.VoteGranted
			return
		}
		if r.VoteGranted {
			voted++
		}
		//		fmt.Printf("voted: %v %v %v\n" ,voted, reply.VoteGranted, reply.Term)
		if voted > len(rf.peers)/2 {
			reply.Term = args.Term
			reply.VoteGranted = true
			fmt.Printf("DEBUG request votes verdict %d/%d\n", voted, len(rf.peers))
			return
		}
	}
	fmt.Printf("DEBUG request votes verdict %d/%d\n", voted, len(rf.peers))
	reply.Term = args.Term
	reply.VoteGranted = false
}

// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	if !ok {
		log.Printf("Could not request vote from server %d: %s ", server, rf.string())
	}
	return ok
}

type AppendEntriesArgs struct {
	Term int
	LeaderId int
}

type AppendEntriesReply struct {
	Term int
	Success bool
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	atomic.StoreInt32(&rf.heartbeat, 1);

	if args.Term > rf.currentTerm {
		rf.mu.Lock()
		rf.set(FOLLOWER)
		rf.setTerm(args.Term)
		rf.mu.Unlock()
		return
	}

	reply.Term = rf.currentTerm
	reply.Success = true
	return
}

func (rf *Raft) appendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	ch := make(chan *AppendEntriesReply)
	for i := range(rf.peers) {
		if i == rf.me {
			continue; // make sure to skip itself to avoid deadlock
		}
		go func(i int) {
			var r AppendEntriesReply
			if rf.sendAppendEntries(i, args, &r){
				ch <- &r
			} else {
				ch <- &AppendEntriesReply{Success: false, Term: -1}
			}
		}(i)
	}
	success := 1
	for i := 0; i < len(rf.peers); i++ {
		r := <-ch
		if (r.Term > args.Term) {
			reply.Term = r.Term
			reply.Success = r.Success
			return
		}
		if r.Success {
			success++
		}
		if success > len(rf.peers)/2 {
			reply.Term = args.Term
			reply.Success = true
			return
		}
	}
	reply.Term = args.Term
	reply.Success = false
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	if !ok {
		log.Printf("Could not append entries to server %d: %s ", server, rf.string())
	}
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).


	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		ms := rf.tick()
		time.Sleep(ms)
	}
}

func (rf *Raft) tick() time.Duration {
	fmt.Println("DEBUG tick: " + rf.string())
	switch rf.get() {
	case LEADER:
		var reply AppendEntriesReply
		rf.mu.Lock()
		rf.appendEntries(&AppendEntriesArgs{
			Term: rf.currentTerm,
			LeaderId: rf.me,
		}, &reply)
		if rf.get() != LEADER || reply.Term > rf.currentTerm {
			rf.setTerm(reply.Term)
			rf.set(FOLLOWER)
			rf.mu.Unlock()
			return randomSleep()
		} else {
			rf.mu.Unlock()
			return sleep()
		}
	case CANDIDATE:
		var reply RequestVoteReply
		fmt.Println("DEBUG tick requesting votes")
		rf.requestVotes(&RequestVoteArgs{
			Term: rf.currentTerm,
			CandidateId: rf.votedFor,
		}, &reply)
		rf.mu.Lock()
		if rf.get() != CANDIDATE || !reply.VoteGranted || reply.Term > rf.currentTerm {
			rf.setTerm(reply.Term)
			rf.set(FOLLOWER)
			rf.mu.Unlock()
			return randomSleep()
		} else {
			assertf(reply.Term == rf.currentTerm,
				"current term %d does not match request vote reply %d: %s",
				rf.currentTerm,
				reply.Term,
				rf.string())
			rf.set(LEADER)
			rf.mu.Unlock()
			// roll over to leader without pause
			return 0
		}
	case FOLLOWER:
		// Check if a leader election should be started.
		h := atomic.LoadInt32(&rf.heartbeat);
		if h == 0 {
			rf.mu.Lock()
			rf.set(CANDIDATE)
			rf.votedFor = rf.me
			rf.incTerm()
			rf.mu.Unlock()
			// reset election timer if starting election
			atomic.StoreInt32(&rf.heartbeat, 1);
			// roll over to candiate without pause
			return 0
		} else {
			atomic.StoreInt32(&rf.heartbeat, 0);
			return randomSleep()
		}
	}
	panic("tick did not evaluate")
}

func (rf *Raft) set(state string) {
	rf.is = state
}

func (rf *Raft) get() string {
	return rf.is
}

func (rf *Raft) incTerm() {
	rf.setTerm(rf.currentTerm + 1)
}

func (rf *Raft) setTerm(term int) {
	if rf.currentTerm > term {
		log.Printf("cannot set term %d less than current %d: %s", term, rf.currentTerm, rf.string())
		return
	}
	rf.currentTerm = term
}

func randomSleep() time.Duration {
	// Pause for a random amount of time between 50 and 350 ms
	ms := 50 + (rand.Int63() % 300)
	return time.Duration(ms) * time.Millisecond
}

func sleep() time.Duration {
	// Idle sleep for the leader. Must be less than randomSleep
	ms := 25
	return time.Duration(ms) * time.Millisecond
}

func assertf(cond bool, format string, v ...interface{}) {
	if !cond {
		panic(fmt.Sprintf(format, v...))
	}
}

func (rf *Raft) string() string {
	return fmt.Sprintf("[me:%d, is:%s, currentTerm:%d, votedFor: %d]",
		rf.me,
		rf.is,
		rf.currentTerm,
		rf.votedFor,
	)
}


// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.is = FOLLOWER
	rf.votedFor = -1 // TODO: initialization to -1 might not be needed
	rf.heartbeat = 1

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()


	return rf
}
