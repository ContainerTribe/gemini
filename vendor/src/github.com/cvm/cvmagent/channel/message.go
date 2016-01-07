package channel

type Message struct {
	Type    int
	Content interface{}
}

// agent to daemon message
const (
	MSG_AGENT_READY = iota
	MSG_ACK
)

type AgentReadyMessage struct {
}

const (
	ACK_OK = iota
	ACK_ERROR
)

type AckMessage struct {
	AckType int
	AckMsg  string
}

// daemon to agent message
const (
	MSG_ADD_CONTAINER = iota
	MSG_SET_IP
)

type AddContainerMessage struct {
	// container root filesystem
	Rootfs string

	// container entrypoint
	CmdArgs []string

	// Env
	Env []string
}

type SetIPMessage struct {
	// network interface name
	IfName string

	// ip
	IpAddr string

	// netmask
	NetMask string
}
