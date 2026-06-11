package presence

// InitSend initialises the buffered send channel.
// Called by the handler after creating a Client.
func (c *Client) InitSend(bufSize int) {
	c.send = make(chan []byte, bufSize)
}
