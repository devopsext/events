package common

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Event struct {
	Time          string      `json:"time"`
	TimeNano      int64       `json:"timeNano"`
	Orchestration string      `json:"orchestration"`
	Kind          string      `json:"kind"`
	Location      string      `json:"location"`
	Operation     string      `json:"operation"`
	Object        interface{} `json:"object,omitempty"`
	User          *User       `json:"user"`
}
