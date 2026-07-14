package model

import "time"

// InvalidatePricing makes the next public pricing read rebuild immediately.
// It avoids doing database work while an Option update still holds its lock.
func InvalidatePricing() {
	updatePricingLock.Lock()
	lastGetPricingTime = time.Time{}
	updatePricingLock.Unlock()
}

// RefreshPricing 强制立即重新计算与定价相关的缓存。
// 该方法用于需要最新数据的内部管理 API，
// 因此会绕过默认的 1 分钟延迟刷新。
func RefreshPricing() {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	modelSupportEndpointsLock.Lock()
	defer modelSupportEndpointsLock.Unlock()

	updatePricing()
}
