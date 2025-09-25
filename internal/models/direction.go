//Expansion Direction(扩散方向)

package models

// 枚举类型
type DirectionType string

const (
	Broad    DirectionType = "broad"    // 广度扩散
	Deep     DirectionType = "deep"     // 深度扩散
	Lateral  DirectionType = "lateral"  // 横向思维
	Critical DirectionType = "critical" // 批判思维
)

// 结构体
type Direction struct {
	Type        DirectionType
	Title       string
	Description string
	Keywords    []string
	Relevance   float64
}

// 方法
func NewDirection(dirType DirectionType, title, desc string) *Direction {
	return &Direction{
		Type:        dirType,
		Title:       title,
		Description: desc,
		Keywords:    make([]string, 0),
		Relevance:   0,
	}
}

func (d *Direction) AddKeyword(keyword string) {
	if d == nil || keyword == "" {
		return
	}

	for _, existing := range d.Keywords {
		if existing == keyword {
			return
		}
	}
	d.Keywords = append(d.Keywords, keyword)
}

func (d *Direction) SetRelevance(score float64) {
	if d == nil {
		return
	}

	switch {
	case score < 0:
		d.Relevance = 0
	case score > 1:
		d.Relevance = 1
	default:
		d.Relevance = score
	}
}
