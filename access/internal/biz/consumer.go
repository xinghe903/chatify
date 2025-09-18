package biz

import "access/internal/data"

type Consumer struct {
}

func NewConsumer(data *data.Data) *Consumer {
	return &Consumer{}
}
