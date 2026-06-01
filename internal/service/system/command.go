package system

// EchoCommand 表示回显已校验消息的服务层输入。
type EchoCommand struct {
	Message string
	Tag     *string
}
