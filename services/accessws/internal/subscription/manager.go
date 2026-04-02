package subscription

import (
	"github.com/viabtc/go-project/services/accessws/internal/model"
	"strconv"
	"sync"
)

type Manager struct {
	orderSubs      map[uint32]map[*model.ClientSession]bool
	assetSubs      map[string]map[*model.ClientSession]bool
	depthSubs      map[string]map[*model.ClientSession]bool
	klineSubs      map[string]map[*model.ClientSession]bool
	priceSubs      map[string]map[*model.ClientSession]bool
	dealsSubs      map[string]map[*model.ClientSession]bool
	stateSubs      map[string]map[*model.ClientSession]bool
	todaySubs      map[string]map[*model.ClientSession]bool
	depthSnapshots map[string]*model.DepthSnapshot
	depthSnapMu    sync.RWMutex
	dealsBuffers   map[string]*model.DealsBuffer
	dealsBufMu     sync.RWMutex
	mu             sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		orderSubs:      make(map[uint32]map[*model.ClientSession]bool),
		assetSubs:      make(map[string]map[*model.ClientSession]bool),
		depthSubs:      make(map[string]map[*model.ClientSession]bool),
		klineSubs:      make(map[string]map[*model.ClientSession]bool),
		priceSubs:      make(map[string]map[*model.ClientSession]bool),
		dealsSubs:      make(map[string]map[*model.ClientSession]bool),
		stateSubs:      make(map[string]map[*model.ClientSession]bool),
		todaySubs:      make(map[string]map[*model.ClientSession]bool),
		depthSnapshots: make(map[string]*model.DepthSnapshot),
		dealsBuffers:   make(map[string]*model.DealsBuffer),
	}
}

func assetKey(userID uint32, asset string) string {
	return strconv.FormatUint(uint64(userID), 10) + ":" + asset
}

func depthKey(market, interval string, limit int) string {
	return market + ":" + interval + ":" + strconv.Itoa(limit)
}

func klineKey(market, interval string) string {
	return market + ":" + interval
}

func (m *Manager) OrderSubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.orderSubs[sess.UserID] == nil {
		m.orderSubs[sess.UserID] = make(map[*model.ClientSession]bool)
	}
	m.orderSubs[sess.UserID][sess] = true
}

func (m *Manager) OrderUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if subs, ok := m.orderSubs[sess.UserID]; ok {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.orderSubs, sess.UserID)
		}
	}
}

func (m *Manager) GetOrderSubscribers(userID uint32) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.orderSubs[userID]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) AssetSubscribe(sess *model.ClientSession, asset string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := assetKey(sess.UserID, asset)
	if m.assetSubs[key] == nil {
		m.assetSubs[key] = make(map[*model.ClientSession]bool)
	}
	m.assetSubs[key][sess] = true
}

func (m *Manager) AssetUnsubscribe(sess *model.ClientSession, asset string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := assetKey(sess.UserID, asset)
	if subs, ok := m.assetSubs[key]; ok {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.assetSubs, key)
		}
	}
}

func (m *Manager) GetAssetSubscribers(userID uint32, asset string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := assetKey(userID, asset)
	var result []*model.ClientSession
	if subs, ok := m.assetSubs[key]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) DepthSubscribe(sess *model.ClientSession, market, interval string, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := depthKey(market, interval, limit)
	if m.depthSubs[key] == nil {
		m.depthSubs[key] = make(map[*model.ClientSession]bool)
	}
	m.depthSubs[key][sess] = true
}

func (m *Manager) DepthUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, subs := range m.depthSubs {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.depthSubs, key)
		}
	}
}

func (m *Manager) GetDepthSubscribers(key string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.depthSubs[key]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) GetAllDepthSubs() map[string]map[*model.ClientSession]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]map[*model.ClientSession]bool)
	for k, v := range m.depthSubs {
		result[k] = v
	}
	return result
}

func (m *Manager) KlineSubscribe(sess *model.ClientSession, market, interval string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := klineKey(market, interval)
	if m.klineSubs[key] == nil {
		m.klineSubs[key] = make(map[*model.ClientSession]bool)
	}
	m.klineSubs[key][sess] = true
}

func (m *Manager) KlineUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, subs := range m.klineSubs {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.klineSubs, key)
		}
	}
}

func (m *Manager) GetKlineSubscribers(key string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.klineSubs[key]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) GetAllKlineSubs() map[string]map[*model.ClientSession]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]map[*model.ClientSession]bool)
	for k, v := range m.klineSubs {
		result[k] = v
	}
	return result
}

func (m *Manager) PriceSubscribe(sess *model.ClientSession, market string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.priceSubs[market] == nil {
		m.priceSubs[market] = make(map[*model.ClientSession]bool)
	}
	m.priceSubs[market][sess] = true
}

func (m *Manager) PriceUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for market, subs := range m.priceSubs {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.priceSubs, market)
		}
	}
}

func (m *Manager) GetPriceSubscribers(market string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.priceSubs[market]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) GetAllPriceSubs() map[string]map[*model.ClientSession]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]map[*model.ClientSession]bool)
	for k, v := range m.priceSubs {
		result[k] = v
	}
	return result
}

func (m *Manager) DealsSubscribe(sess *model.ClientSession, market string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dealsSubs[market] == nil {
		m.dealsSubs[market] = make(map[*model.ClientSession]bool)
	}
	m.dealsSubs[market][sess] = true
}

func (m *Manager) DealsUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for market, subs := range m.dealsSubs {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.dealsSubs, market)
		}
	}
}

func (m *Manager) GetDealsSubscribers(market string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.dealsSubs[market]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) GetAllDealsSubs() map[string]map[*model.ClientSession]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]map[*model.ClientSession]bool)
	for k, v := range m.dealsSubs {
		result[k] = v
	}
	return result
}

func (m *Manager) StateSubscribe(sess *model.ClientSession, market string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stateSubs[market] == nil {
		m.stateSubs[market] = make(map[*model.ClientSession]bool)
	}
	m.stateSubs[market][sess] = true
}

func (m *Manager) StateUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for market, subs := range m.stateSubs {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.stateSubs, market)
		}
	}
}

func (m *Manager) GetStateSubscribers(market string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.stateSubs[market]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) GetAllStateSubs() map[string]map[*model.ClientSession]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]map[*model.ClientSession]bool)
	for k, v := range m.stateSubs {
		result[k] = v
	}
	return result
}

func (m *Manager) TodaySubscribe(sess *model.ClientSession, market string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.todaySubs[market] == nil {
		m.todaySubs[market] = make(map[*model.ClientSession]bool)
	}
	m.todaySubs[market][sess] = true
}

func (m *Manager) TodayUnsubscribe(sess *model.ClientSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for market, subs := range m.todaySubs {
		delete(subs, sess)
		if len(subs) == 0 {
			delete(m.todaySubs, market)
		}
	}
}

func (m *Manager) GetTodaySubscribers(market string) []*model.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*model.ClientSession
	if subs, ok := m.todaySubs[market]; ok {
		for sess := range subs {
			result = append(result, sess)
		}
	}
	return result
}

func (m *Manager) GetAllTodaySubs() map[string]map[*model.ClientSession]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]map[*model.ClientSession]bool)
	for k, v := range m.todaySubs {
		result[k] = v
	}
	return result
}

func (m *Manager) GetDepthSnapshot(key string) *model.DepthSnapshot {
	m.depthSnapMu.RLock()
	defer m.depthSnapMu.RUnlock()
	return m.depthSnapshots[key]
}

func (m *Manager) SetDepthSnapshot(key string, snap *model.DepthSnapshot) {
	m.depthSnapMu.Lock()
	defer m.depthSnapMu.Unlock()
	m.depthSnapshots[key] = snap
}

func (m *Manager) GetDealsBuffer(market string) *model.DealsBuffer {
	m.dealsBufMu.Lock()
	defer m.dealsBufMu.Unlock()
	if buf, ok := m.dealsBuffers[market]; ok {
		return buf
	}
	buf := model.NewDealsBuffer(100)
	m.dealsBuffers[market] = buf
	return buf
}

func (m *Manager) UpdateDealsBuffer(market string, deal model.DealRecord) {
	buf := m.GetDealsBuffer(market)
	buf.Add(deal)
}
