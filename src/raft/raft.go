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
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

const (
	FOLLOWER = "FOLLOWER"
	CANDIDATE = "CANDIDATE"
	LEADER = "LEADER"
)

const RPC_TIMEOUT = 500


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
	state  		string
	heartbeat 	int
	currentTerm int
	votedFor  	int
	logs        []*Log
	lastApplied int
	commitIndex int
	nextIndex  	[]int
	matchIndex 	[]int
	applyCh chan ApplyMsg
}

func (rf *Raft) getLog(index int) *Log {
	if index < 0 || index >= len(rf.logs) {
		return nil
	}
	return rf.logs[index]
}

func (rf *Raft) insertLog(index int, term int, entries []*Log) int {
    // TODO: might need to check conflict, receiver impl #3
	rf.logs = append(rf.logs[0:index], entries...)
	return len(rf.logs)
}

type Log struct {
	Term int
	Command interface{}
}

func (rf *Raft) string() string {
	return fmt.Sprintf("{me: %v, st: %v, hb: %v, ct: %v, vf: %v}",
		rf.me,
		rf.state,
		rf.heartbeat,
		rf.currentTerm,
		rf.votedFor,
	)
}

// assumes the caller obtained the lock
func (rf *Raft) initLeader() {
	for i := range(rf.nextIndex) {
		rf.nextIndex[i] = len(rf.logs)
	}
	for i := range(rf.matchIndex) {
		rf.matchIndex[i] = 0
	}
}

// assumes the caller obtained the lock
func (rf *Raft) matchIndexMajority() int {
	m := make(map[int]int)
	for i := range(rf.matchIndex) {
		m[rf.matchIndex[i]]++
	}
	freq := 0
	term := -1
	for t, f := range(m) {
		if (f > freq) {
			freq = f
			term = t
		}
	}
	return term
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// Your code here (2A).
	return rf.currentTerm, rf.state == LEADER
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

type AppendEntriesArgs struct {
	Term int
	LeaderId int
	PrevLogIndex int
	PrevLogTerm int
	Entries []*Log
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term int
	Success bool
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	rf.heartbeat = 1

	if args.Term > rf.currentTerm {
		rf.state = FOLLOWER
		rf.currentTerm = args.Term
	}

	reply.Term = rf.currentTerm
	reply.Success = true
	return
}

func (rf *Raft) sendEntries(args *AppendEntriesArgs) int {
	ch := make(chan *AppendEntriesReply)
	rf.mu.Lock()
	for i := range(rf.peers) {
		if i == rf.me {
			continue; // skip itself
		}
		go func(
			peer int,
			lastIndex int,
			nextIndex int) {
			for lastIndex >= nextIndex - 1 {
				entries := []*Log{}
				prevLogIndex := nextIndex - 1
				prevLogTerm := -1
				if prevLogIndex >= 0 {
					prevLogIndex = rf.logs[prevLogIndex].Term
				}
				if lastIndex >= nextIndex {
					entries = append(entries, rf.logs[nextIndex])
				}
				var r AppendEntriesReply
				if rf.sendAppendEntries(peer, &AppendEntriesArgs{
					Term: args.Term,
					LeaderId: args.LeaderId,
					LeaderCommit: args.LeaderCommit,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm: prevLogTerm,
					Entries: entries,
				}, &r){
					if r.Term > args.Term {
						ch <- &AppendEntriesReply{Success: false, Term: r.Term}
						return
					}

					if r.Success {
						if lastIndex >= nextIndex {
							nextIndex++
						}
						if (lastIndex < nextIndex) {
							rf.mu.Lock()
							if (rf.nextIndex[peer] < nextIndex) {
								rf.nextIndex[peer] = nextIndex
							}
							if (rf.matchIndex[peer] < nextIndex - 1) {
								rf.matchIndex[peer] = nextIndex - 1
							}
							rf.mu.Unlock()
							ch <- &r
							return
						}
					} else {
						nextIndex--
						if (nextIndex < 0) {
							panic("nextIndex less than 0")
						}
					}
				} else {
					ch <- &AppendEntriesReply{Success: false, Term: -1}
				}
			}
		}(i, len(rf.logs) - 1, rf.nextIndex[i])
	}
	rf.mu.Unlock()
	success := 1
	for i := 0; i < len(rf.peers) - 1; i++ {
		select {
		case r := <-ch:
			if (r.Term > args.Term) {
				return r.Term
			}
			if r.Success {
				success++
			}
			if success > len(rf.peers)/2 {
				return args.Term
			}
		case <-time.After(RPC_TIMEOUT * time.Millisecond):
			continue
		}
	}
	return args.Term
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	Term int
	CandidateId int
	lastLogIndex int
	lastLogTerm int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	Term int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.state = FOLLOWER
		rf.currentTerm = args.Term
		rf.votedFor = args.CandidateId
		rf.heartbeat = 1  // reset election timer if vote granted
		reply.Term = args.Term
		reply.VoteGranted = true
		//		fmt.Println("vote granted " + rf.string() )
		return
	}

	if rf.votedFor == args.CandidateId && rf.currentTerm == args.Term {
		reply.Term = rf.currentTerm
		reply.VoteGranted = true
		return
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false
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
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.

func (rf *Raft) startElection(
	candidate int,
	term int,
	lastIndex int,
	lastTerm int) (int, bool) {
	args := &RequestVoteArgs{
		CandidateId: candidate,
		Term: term,
		lastLogIndex: lastIndex,
		lastLogTerm: lastTerm,
	}
	ch := make(chan *RequestVoteReply)
	for i := range(rf.peers) {
		if (i == rf.me) {
			continue; // skip itself
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
	for i := 0; i < len(rf.peers) - 1; i++ {
		select {
		case r := <-ch:
			if (r.Term < args.Term){
				continue;
			}
			if (r.Term > args.Term) {
				return r.Term, false
			}
			if r.VoteGranted {
				voted++
			}
			if voted > len(rf.peers)/2 {
				return r.Term, true
			}
		case <-time.After(RPC_TIMEOUT * time.Millisecond):
			continue
		}
	}
	return args.Term, false
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	index := len(rf.logs)
	term := rf.currentTerm
	isLeader := rf.state == LEADER

	if isLeader {
		rf.logs = append(rf.logs, &Log{Term: rf.currentTerm, Command: command})
		rf.nextIndex[rf.me] = index + 1
		rf.matchIndex[rf.me] = index
		go rf.startAgreement(&AppendEntriesArgs{
			Term: rf.currentTerm,
			LeaderId: rf.me,
			LeaderCommit: rf.commitIndex,
		})
	}

	return index, term, isLeader
}

func (rf *Raft) startAgreement(args *AppendEntriesArgs) {
	term := rf.sendEntries(args)
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if term > rf.currentTerm {
		rf.currentTerm = maxInt(term, rf.currentTerm + 1)
		rf.votedFor = -1
		rf.state = FOLLOWER
		rf.heartbeat = 1
		return
	}

	// determine commitIndex from the matchIndex majority
	rf.commitIndex = maxInt(rf.commitIndex, rf.matchIndexMajority())
	for ;rf.lastApplied < rf.commitIndex; rf.lastApplied++ {
		rf.applyCh <- ApplyMsg{
			CommandValid: false,
			Command: rf.logs[rf.lastApplied+1],
			CommandIndex: rf.lastApplied+1,
		}
	}
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
		// Check if a leader election should be started.
		fmt.Printf("DEBUG tick %s\n", rf.string())
		ms := rf.tick()
		time.Sleep(ms)
	}
}

func (rf *Raft) tick() time.Duration {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	switch rf.state {
	case FOLLOWER:
		if rf.heartbeat == 0 {
			rf.state = CANDIDATE
			rf.currentTerm++
			rf.votedFor = rf.me
			return 0
		} else {
			rf.heartbeat = 0;
			return rsleep()
		}
	case CANDIDATE:
		currentTerm := rf.currentTerm
		votedFor := rf.votedFor
		lastIndex := len(rf.logs) - 1
		lastTerm := -1
		if (lastIndex >= 0) {
			lastTerm = rf.logs[lastIndex].Term
		}
		rf.mu.Unlock()
		term, elected := rf.startElection(
			votedFor,
			currentTerm,
			lastIndex,
			lastTerm)
		rf.mu.Lock()
		if (currentTerm != rf.currentTerm) {
			return 0
		}
		if term > rf.currentTerm || !elected {
			rf.currentTerm = maxInt(term, rf.currentTerm + 1)
			rf.votedFor = -1
			rf.state = FOLLOWER
			rf.heartbeat = 1
			return 0
		}
		rf.state = LEADER
		rf.heartbeat = 1
		rf.initLeader()
		return 0
	case LEADER:
		currentTerm := rf.currentTerm
		leaderCommit := rf.commitIndex
		leader := rf.me
		rf.mu.Unlock()
		term := rf.sendEntries(&AppendEntriesArgs{
			Term: currentTerm,
			LeaderId: leader,
			LeaderCommit: leaderCommit,
		})
		rf.mu.Lock()
		if (currentTerm != rf.currentTerm) {
			return 0
		}
		if term > rf.currentTerm {
			rf.currentTerm = maxInt(term, rf.currentTerm + 1)
			rf.votedFor = -1
			rf.state = FOLLOWER
			rf.heartbeat = 1
			return 0
		}
		return sleep()
	}
	panic("did not evaluate: " + rf.string())
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func sleep() time.Duration {
	ms := 25
	return time.Duration(ms) * time.Millisecond
}

func rsleep() time.Duration {
	ms := 50 + (rand.Int63() % 300)
	return time.Duration(ms) * time.Millisecond
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
	rf.state = FOLLOWER
	rf.heartbeat = 1
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.lastApplied = -1
	rf.commitIndex = 0
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
