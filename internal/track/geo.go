package track

import (
	"net/netip"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// geoResolver 基于 ip2region xdb 的离线归属地查询。
// 仅被 Tracker 的 worker goroutine 调用，无需加锁。
type geoResolver struct {
	s *xdb.Searcher
}

func newGeoResolver(xdbPath string) (*geoResolver, error) {
	vIndex, err := xdb.LoadVectorIndexFromFile(xdbPath)
	if err != nil {
		return nil, err
	}
	s, err := xdb.NewWithVectorIndex(xdb.IPv4, xdbPath, vIndex)
	if err != nil {
		return nil, err
	}
	return &geoResolver{s: s}, nil
}

func (g *geoResolver) close() { g.s.Close() }

// lookup 返回 (country, province, city)。xdb 地域串格式为
// "国家|区域|省份|城市|ISP"，字段为 "0" 表示未知。
func (g *geoResolver) lookup(ip string) (country, province, city string) {
	region, err := g.s.Search(ip)
	if err != nil {
		return "", "", ""
	}
	parts := strings.Split(region, "|")
	field := func(i int) string {
		if i < len(parts) && parts[i] != "0" {
			return parts[i]
		}
		return ""
	}
	return field(0), field(2), field(3)
}

// isInternalIP 判定内网/本机地址 (无需 xdb，纯 netip)。
func isInternalIP(ip string) bool {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsUnspecified()
}

// resolveGeo 解析 IP 归属地 (country, province, city)，带按 IP 缓存。
// 内网 IP 标记 country="内网"；IPv6 及无 xdb 时的公网 IP 留空。
func resolveGeo(cache map[string][3]string, geo *geoResolver, ip string) [3]string {
	if r, ok := cache[ip]; ok {
		return r
	}
	var r [3]string
	switch {
	case isInternalIP(ip):
		r = [3]string{"内网", "", ""}
	case geo != nil:
		if a, err := netip.ParseAddr(ip); err == nil && a.Unmap().Is4() {
			c, p, city := geo.lookup(ip)
			r = [3]string{c, p, city}
		}
	}
	cache[ip] = r
	return r
}
