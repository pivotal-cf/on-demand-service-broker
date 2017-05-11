// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("tasks", func() {
	Describe("deserialization", func() {
		It("reads tasks from json", func() {
			data := []byte(`[
	  {
	    "id": 12986,
	    "state": "done",
	    "description": "snapshot deployment",
	    "timestamp": 1461135602,
	    "result": "snapshots of deployment 'redis-on-demand-broker-dev2' created",
	    "user": "scheduler",
	    "deployment": "redis-on-demand-broker-dev2"
	  },
	  {
	    "id": 12729,
	    "state": "done",
	    "description": "snapshot deployment",
	    "timestamp": 1461049202,
	    "result": "snapshots of deployment 'redis-on-demand-broker-dev2' created",
	    "user": "scheduler",
	    "deployment": "redis-on-demand-broker-dev2"
	  },
	  {
	    "id": 12427,
	    "state": "done",
	    "description": "snapshot deployment",
	    "timestamp": 1460962800,
	    "result": "snapshots of deployment 'redis-on-demand-broker-dev2' created",
	    "user": "scheduler",
	    "deployment": "redis-on-demand-broker-dev2"
	  }]`)
			var tasks boshdirector.BoshTasks
			Expect(json.Unmarshal(data, &tasks)).To(Succeed())

			Expect(tasks).To(Equal(boshdirector.BoshTasks{
				{
					ID:          12986,
					State:       "done",
					Description: "snapshot deployment",
					Result:      "snapshots of deployment 'redis-on-demand-broker-dev2' created",
				},
				{
					ID:          12729,
					State:       "done",
					Description: "snapshot deployment",
					Result:      "snapshots of deployment 'redis-on-demand-broker-dev2' created",
				},
				{
					ID:          12427,
					State:       "done",
					Description: "snapshot deployment",
					Result:      "snapshots of deployment 'redis-on-demand-broker-dev2' created",
				},
			}))
		})
	})
	Describe("IncompleteTasks", func() {
		Context("when one task is inprogress", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{ID: 1, State: boshdirector.BoshTaskProcessing},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
			}

			It("reports one incomplete task", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(1))
			})

			It("returns inprogress task", func() {
				Expect(boshTasks.IncompleteTasks()[0].ID).To(Equal(1))
			})
		})

		Context("when one task is queued", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{ID: 1, State: boshdirector.BoshTaskQueued},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
			}

			It("reports one incomplete task", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(1))
			})

			It("returns queued task", func() {
				Expect(boshTasks.IncompleteTasks()[0].ID).To(Equal(1))
			})
		})

		Context("when one task is cancelling", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{ID: 1, State: boshdirector.BoshTaskCancelling},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
			}

			It("reports one incomplete task", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(1))
			})

			It("returns cancelling task", func() {
				Expect(boshTasks.IncompleteTasks()[0].ID).To(Equal(1))
			})
		})

		Context("when all tasks are inprogress", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{ID: 1, State: boshdirector.BoshTaskProcessing},
				boshdirector.BoshTask{ID: 2, State: boshdirector.BoshTaskProcessing},
				boshdirector.BoshTask{ID: 3, State: boshdirector.BoshTaskProcessing},
			}

			It("returns all incomplete tasks", func() {
				Expect(boshTasks.IncompleteTasks()).To(Equal(boshTasks))
			})

		})

		Context("on a list of done tasks", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
			}

			It("returns no incomplete tasks", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(0))
			})
		})

		Context("on a list of cancelled tasks", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskCancelled},
				boshdirector.BoshTask{State: boshdirector.BoshTaskCancelled},
				boshdirector.BoshTask{State: boshdirector.BoshTaskCancelled},
			}

			It("returns no incomplete tasks", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(0))
			})
		})

		Context("on a list of timed out tasks", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskTimeout},
				boshdirector.BoshTask{State: boshdirector.BoshTaskTimeout},
				boshdirector.BoshTask{State: boshdirector.BoshTaskTimeout},
			}

			It("returns no incomplete tasks", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(0))
			})
		})

		Context("on a list of errored tasks", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
			}

			It("returns no incomplete tasks", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(0))
			})
		})

		Context("when there are no tasks", func() {
			boshTasks := boshdirector.BoshTasks{}

			It("return no tasks", func() {
				Expect(boshTasks.IncompleteTasks()).To(HaveLen(0))
			})
		})
	})

	Describe("DoneTasks", func() {
		Context("when all tasks are done", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
			}

			It("returns all done tasks", func() {
				Expect(boshTasks.DoneTasks()).To(HaveLen(3))
			})
		})

		Context("when no tasks are done", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
			}

			It("returns no tasks", func() {
				Expect(boshTasks.DoneTasks()).To(HaveLen(0))
			})
		})

		Context("when one task is done", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
			}

			It("returns one task", func() {
				Expect(boshTasks.DoneTasks()).To(HaveLen(1))
			})
		})

		Context("when there are no tasks", func() {
			boshTasks := boshdirector.BoshTasks{}

			It("return no tasks", func() {
				Expect(boshTasks.DoneTasks()).To(HaveLen(0))
			})
		})
	})

	Describe("ErrorTasks", func() {
		Context("when all tasks have errored", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
			}

			It("returns all error tasks", func() {
				Expect(boshTasks.FailedTasks()).To(HaveLen(3))
			})
		})

		Context("when no tasks have errored", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
			}

			It("returns no tasks", func() {
				Expect(boshTasks.FailedTasks()).To(HaveLen(0))
			})
		})

		Context("when one task has errored", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskError},
			}

			It("returns one task", func() {
				Expect(boshTasks.FailedTasks()).To(HaveLen(1))
			})
		})

		Context("when a task has been cancelled", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskCancelled},
			}

			It("returns one task", func() {
				Expect(boshTasks.FailedTasks()).To(HaveLen(1))
			})
		})

		Context("when a task has timed out", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{State: boshdirector.BoshTaskTimeout},
			}

			It("returns one task", func() {
				Expect(boshTasks.FailedTasks()).To(HaveLen(1))
			})
		})

		Context("when there are no tasks", func() {
			boshTasks := boshdirector.BoshTasks{}

			It("return no tasks", func() {
				Expect(boshTasks.FailedTasks()).To(HaveLen(0))
			})
		})
	})

	Describe("ToLog", func() {
		Context("when there are no tasks", func() {
			boshTasks := boshdirector.BoshTasks{}

			It("returns one task in log format", func() {
				Expect(boshTasks.ToLog()).To(Equal("[]"))
			})
		})

		Context("when there is one task", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{ID: 1, State: boshdirector.BoshTaskProcessing},
			}

			It("returns one task in log format", func() {
				Expect(boshTasks.ToLog()).To(Equal(
					fmt.Sprintf("[%s]", boshTasks[0].ToLog()),
				))
			})
		})

		Context("when there are several tasks", func() {
			boshTasks := boshdirector.BoshTasks{
				boshdirector.BoshTask{ID: 1, State: boshdirector.BoshTaskDone},
				boshdirector.BoshTask{ID: 2, State: boshdirector.BoshTaskTimeout},
				boshdirector.BoshTask{ID: 3, State: boshdirector.BoshTaskProcessing},
			}

			It("returns all three tasks in log format", func() {
				Expect(boshTasks.ToLog()).To(Equal(
					fmt.Sprintf(
						"[%s,%s,%s]",
						boshTasks[0].ToLog(),
						boshTasks[1].ToLog(),
						boshTasks[2].ToLog(),
					),
				))
			})
		})
	})
})
