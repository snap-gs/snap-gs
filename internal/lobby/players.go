package lobby

import (
	"strconv"
	"sync"
)

type Players struct {
	x     sync.RWMutex
	bots  map[int64]bool
	joins map[int64]*Player
	admin *Player
}

type Player struct {
	id   int64
	name string
	uuid string
}

func (p *Players) migrate(id int64) bool {
	if p.admin == nil || id != p.admin.id {
		return false
	}
	p.admin = nil
	for _, player := range p.joins {
		if p.admin == nil || player.id < p.admin.id {
			p.admin = player
		}
	}
	return true
}

func (p *Players) set(sid, name, uuid string) (int64, string, string, bool) {
	id, _ := strconv.ParseInt(sid, 10, 64)
	if id < 1 {
		return -1, "", "", false
	}
	if id < 1000 {
		if p.bots == nil {
			p.bots = make(map[int64]bool, 10)
		}
		p.bots[id] = true
		return id, "", "", false
	}
	player := p.joins[id]
	if player == nil {
		player = &Player{id: id}
	}
	if name != "" {
		player.name = name
	}
	if uuid != "" {
		player.uuid = uuid
	}
	if len(p.joins) == 0 {
		p.admin = player
	}
	if p.joins == nil {
		p.joins = make(map[int64]*Player, 15)
	}
	p.joins[id] = player
	return player.id, player.name, player.uuid, player.id == p.admin.id
}

func (p *Players) Add(id string) (int64, string, string, bool) {
	p.x.Lock()
	defer p.x.Unlock()
	return p.set(id, "", "")
}

func (p *Players) Update(id, name, uuid string) (int64, string, string, bool) {
	p.x.Lock()
	defer p.x.Unlock()
	return p.set(id, name, uuid)
}

func (p *Players) Remove(id string) (int64, string, string, bool) {
	p.x.Lock()
	defer p.x.Unlock()
	i, _ := strconv.ParseInt(id, 10, 64)
	if _, ok := p.bots[i]; ok {
		delete(p.bots, i)
		return i, "", "", false
	}
	if player, ok := p.joins[i]; ok {
		delete(p.joins, i)
		return i, player.name, player.uuid, p.migrate(i)
	}
	return -1, "", "", false
}

func (p *Players) Lookup(id string) (int64, string, string, bool) {
	p.x.RLock()
	defer p.x.RUnlock()
	i, _ := strconv.ParseInt(id, 10, 64)
	if player := p.joins[i]; player != nil {
		return player.id, player.name, player.uuid, player.id == p.admin.id
	}
	return -1, "", "", false
}

func (p *Players) Count() (int, int) {
	p.x.RLock()
	defer p.x.RUnlock()
	return len(p.joins), len(p.bots)
}
