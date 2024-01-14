package mr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCoordinator(t *testing.T) {
	c := MakeCoordinator([]string{"A", "B", "C"}, 3)

	// Test work retrival
	taskA := c.MapTasks.GetWork()
	assert.Equal(t, IN_PROGRESS, taskA.State)
	assert.Equal(t, "A", taskA.Key)

	taskB := c.MapTasks.GetWork()
	assert.Equal(t, IN_PROGRESS, taskB.State)
	assert.Equal(t, "B", taskB.Key)

	taskC := c.MapTasks.GetWork()
	assert.Equal(t, IN_PROGRESS, taskC.State)
	assert.Equal(t, "C", taskC.Key)

	// Test work completion
	taskA.State = DONE
	taskB.State = DONE
	taskC.State = DONE

	nilTask := c.MapTasks.GetWork()
	assert.Nil(t, nilTask)
	assert.True(t, c.MapTasks.isDone())

	// Test work timeout
	taskB.State = IN_PROGRESS
	nilTask = c.MapTasks.GetWork()
	assert.Nil(t, nilTask)
	assert.False(t, c.MapTasks.isDone())
	taskB.StartTime = time.Now().Add(- 10 * time.Second)
	c.MapTasks.EnforceTimeout()
	assert.Equal(t, IDLE, taskB.State)
	taskB = c.MapTasks.GetWork()
	assert.Equal(t, IN_PROGRESS, taskB.State)
	assert.Equal(t, "B", taskB.Key)
}
