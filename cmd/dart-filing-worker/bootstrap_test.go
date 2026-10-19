package main

import (
	"errors"

	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/hibiken/asynq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordedEnqueuer struct {
	types []string
	err   error
}

func (f *recordedEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.types = append(f.types, task.Type())
	return nil, f.err
}

var _ = Describe("Worker bootstrap", func() {
	var client *recordedEnqueuer
	BeforeEach(func() { client = &recordedEnqueuer{} })
	Context("when starting or restarting normally", func() {
		DescribeTable("leaves existing queues alone", func(setting string) {
			Expect(bootstrapFetchReports(client, setting)).To(Succeed())
			Expect(client.types).To(BeEmpty())
		}, Entry("unset", ""), Entry("disabled", "false"), Entry("zero", "0"))
	})
	Context("when bootstrap is explicitly enabled", func() {
		It("enqueues one fetch without inspecting or deleting existing tasks", func() {
			Expect(bootstrapFetchReports(client, "true")).To(Succeed())
			Expect(client.types).To(Equal([]string{tasks.TypeTaskFetchReports}))
		})
		It("surfaces enqueue failures", func() {
			client.err = errors.New("queue offline")
			Expect(bootstrapFetchReports(client, "true")).To(MatchError("queue offline"))
		})
	})
	Context("when bootstrap setting is invalid", func() {
		It("fails before touching the queue", func() {
			Expect(bootstrapFetchReports(client, "sometimes")).To(HaveOccurred())
			Expect(client.types).To(BeEmpty())
		})
	})
})
