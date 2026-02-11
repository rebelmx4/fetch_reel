package engine

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/grafov/m3u8"
)

type HLSParser struct{}

func (p *HLSParser) GetHighestQualityURL(masterURL string) (string, error) {
	resp, err := http.Get(masterURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return "", fmt.Errorf("解码 m3u8 失败: %v", err)
	}

	switch listType {
	case m3u8.MASTER:
		master := playlist.(*m3u8.MasterPlaylist)
		if len(master.Variants) == 0 {
			return masterURL, nil
		}

		// 修复点：显式指定返回类型 bool
		sort.Slice(master.Variants, func(i, j int) bool {
			return master.Variants[i].Bandwidth > master.Variants[j].Bandwidth
		})

		bestVariant := master.Variants[0]
		return p.resolveURL(masterURL, bestVariant.URI), nil

	case m3u8.MEDIA:
		return masterURL, nil
	}

	return masterURL, nil
}

func (p *HLSParser) resolveURL(base, ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if u.IsAbs() {
		return ref
	}
	baseU, err := url.Parse(base)
	if err != nil {
		return ref
	}
	return baseU.ResolveReference(u).String()
}
