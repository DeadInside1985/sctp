package sctp

import "sort"

type dataChannelMessageArray []*dataChannelMessage

func (s dataChannelMessageArray) search(seqNum uint16) (*dataChannelMessage, bool) {
	i := sort.Search(len(s), func(i int) bool {
		return s[i].seqNum >= seqNum
	})

	if i < len(s) && s[i].seqNum == seqNum {
		return s[i], true
	}

	return nil, false
}

func (s dataChannelMessageArray) sort() {
	sort.Slice(s, func(i, j int) bool { return s[i].seqNum < s[j].seqNum })
}

type dataChannelMessage struct {
	seqNum        uint16
	fragmentQueue []*chunkPayloadData
	length        int
}

func (m *dataChannelMessage) complete() bool {

	if len(m.fragmentQueue) == 0 {
		// this should be impossible
		return false
	}

	firstPacket := m.fragmentQueue[0]
	if len(m.fragmentQueue) == 1 {
		return firstPacket.beginingFragment && firstPacket.endingFragment
	}

	lastPacket := m.fragmentQueue[len(m.fragmentQueue)-1]
	return firstPacket.beginingFragment && lastPacket.endingFragment

}

func (m *dataChannelMessage) clear() {
	m.length = 0
	m.fragmentQueue = []*chunkPayloadData{}
}

func (m *dataChannelMessage) assemble() ([]byte, bool) {
	if m.complete() {
		b := make([]byte, m.length)
		i := 0
		for _, p := range m.fragmentQueue {
			copy(b[i:], p.userData)
			i += len(p.userData)
		}

		return b, true
	}

	return nil, false
}

type reassemblyQueue struct {
	messageQueue     dataChannelMessageArray
	unorderedMessage dataChannelMessage
	expectedSeqNum   uint16
}

func (r *reassemblyQueue) push(p *chunkPayloadData) {
	if p.unordered {
		r.unorderedMessage.fragmentQueue = append(r.unorderedMessage.fragmentQueue, p)
		r.unorderedMessage.length += len(p.userData)
		return
	}

	m, ok := r.messageQueue.search(p.streamSequenceNumber)
	if !ok {
		m = &dataChannelMessage{seqNum: p.streamSequenceNumber}
		r.messageQueue = append(r.messageQueue, m)
		r.messageQueue.sort()
	}

	m.fragmentQueue = append(m.fragmentQueue, p)
	m.length += len(p.userData)
}

func (r *reassemblyQueue) pop() ([]byte, bool) {

	b, ok := r.unorderedMessage.assemble()
	if ok {
		r.unorderedMessage.clear()
		return b, true
	}

	// Is there any chance that if the message was in the queue, it wouldn't be
	// the first message in the queue?
	if len(r.messageQueue) > 0 {
		m := r.messageQueue[0]
		// Most likely to be true
		if m.seqNum == r.expectedSeqNum {
			b, ok := m.assemble()
			if ok {
				r.messageQueue = r.messageQueue[1:]
				r.expectedSeqNum++
				return b, true
			}

		}
	}
	return nil, false
}
