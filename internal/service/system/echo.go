package system

import "context"

// Echo 返回已校验消息和服务端时间戳。
func (s *Service) Echo(ctx context.Context, command EchoCommand) EchoResult {
	return EchoResult{
		Message:  command.Message,
		Tag:      command.Tag,
		EchoedAt: s.clock.Now(),
	}
}
