package util

import tea "github.com/charmbracelet/bubbletea"

type MessageBus interface {
	Send(msg tea.Msg)

	SetProgram(program *tea.Program)
}

type MessageBusImpl struct {
	program *tea.Program
}

func (m MessageBusImpl) Send(msg tea.Msg) {
	m.program.Send(msg)
}

func (m *MessageBusImpl) SetProgram(program *tea.Program) {
	m.program = program
}

func NewMessageBus(program *tea.Program) MessageBus {
	return &MessageBusImpl{program: program}
}
