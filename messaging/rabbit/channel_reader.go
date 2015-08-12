package rabbit

import "github.com/smartystreets/pipeline/messaging"

type ChannelReader struct {
	controller       Controller
	queue            string
	bindings         []string
	control          chan interface{}
	acknowledgements chan interface{}
	deliveries       chan messaging.Delivery
	shutdown         bool
	deliveryCount    uint64
}

func newReader(controller Controller, queue string, bindings []string) *ChannelReader {
	return &ChannelReader{
		controller:       controller,
		queue:            queue,
		bindings:         bindings,
		control:          make(chan interface{}, 32),
		acknowledgements: make(chan interface{}, 1024),
		deliveries:       make(chan messaging.Delivery, 1024),
	}
}

func (this *ChannelReader) Listen() {
	acknowledger := newAcknowledger(this.control, this.acknowledgements)
	go acknowledger.Listen()

	for this.listen() {
	}

	this.controller.removeReader(this)
}
func (this *ChannelReader) listen() bool {
	channel := this.controller.openChannel()
	if channel == nil {
		return false // broker no longer allowed to give me a channel, it has been manually closed
	}

	subscription := this.subscribe(channel)

	for element := range this.control {
		switch item := element.(type) {
		case shutdownRequested:
			this.shutdown = true
			subscription.Close()
		case subscriptionClosed:
			this.deliveryCount += item.Deliveries
			if this.shutdown {
				// keep channel alive and gracefully stop acknowledgement
				this.acknowledgements <- subscriptionClosed{Deliveries: this.deliveryCount}
				this.deliveryCount = 0
			} else {
				channel.Close() // channel failure; reconnect
				return true
			}
		case acknowledgementCompleted:
			close(this.deliveries)
			channel.Close() // we don't need the channel anymore
			return false    // the shutdown process for this reader is complete
		}
	}

	return true
}
func (this *ChannelReader) subscribe(channel Channel) *Subscription {
	subscription := newSubscription(channel, this.queue, this.bindings, this.control, this.deliveries)
	go subscription.Listen()
	return subscription
}

func (this *ChannelReader) Close() {
	this.control <- shutdownRequested{}
}

func (this *ChannelReader) Deliveries() <-chan messaging.Delivery {
	return this.deliveries
}
func (this *ChannelReader) Acknowledgements() chan<- interface{} {
	return this.acknowledgements
}