package zmap3base

const (
	nilIdx int32 = -1 // NilIdx()
)

func NilIdx() int32 { return nilIdx }

type color uint8

const (
	red   color = 0
	black color = 1
)

// 统一的 root 归一化：RBTree 只认识两种 root：nilIdx(-1) 或 >=0 的真实节点索引。
func normalizeRoot(r int32) int32 {
	if r < 0 {
		return nilIdx
	}
	return r
}

// RichRangeNode ：数组索引版红黑区间树节点（interval tree）
type RichRangeNode struct {
	Range               RichRange
	left, right, parent int32  // 左右孩子与父
	color               color  // 红/黑
	maxEnd              uint16 // 子树内最大 End
}

// NodePool ：节点池（数组 + free list）
// 后面要按 Env 管理内存
type NodePool struct {
	nodes    []RichRangeNode // 真正存节点的数组
	freeHead int32           // 用 nodes[idx].left 串 free list
}

func NewNodePool(capHint int) *NodePool {
	if capHint < 0 {
		capHint = 0
	}

	newNP := GetNodePoolFromPool()
	newNP.nodes = _getGlobalRichRangeNodeSlice(capHint)
	newNP.freeHead = nilIdx
	return newNP
}

func (p *NodePool) Nodes() []RichRangeNode { return p.nodes }

// alloc 返回新节点索引
func (p *NodePool) alloc(rr RichRange) int32 {
	var idx int32
	if p.freeHead != nilIdx {
		idx = p.freeHead
		p.freeHead = p.nodes[idx].left // left 作为 nextFree
	} else {
		idx = int32(len(p.nodes))
		p.nodes = append(p.nodes, RichRangeNode{})
	}

	n := &p.nodes[idx]
	*n = RichRangeNode{
		Range:  rr,
		left:   nilIdx,
		right:  nilIdx,
		parent: nilIdx,
		color:  red,
		maxEnd: rr.End,
	}
	return idx
}

func (p *NodePool) free(idx int32) {
	if idx < 0 {
		return
	}
	// 挂回 free list；left 作为 nextFree
	p.nodes[idx].left = p.freeHead
	p.freeHead = idx
}

// comparator：按 (Begin, End, Accessory) 排序，保证 BST 有确定顺序
// Begin 小的排前
// Begin 相同，End 小的排前
// Begin、End 相同，再比 Accessory（Texture+Config）
func cmpRichRange(a, b RichRange) int {
	ab, bb := a.Begin, b.Begin
	if ab < bb {
		return -1
	}
	if ab > bb {
		return 1
	}
	ae, be := a.End, b.End
	if ae < be {
		return -1
	}
	if ae > be {
		return 1
	}
	au, bu := a.Accessory.IntoUint64(), b.Accessory.IntoUint64()
	if au < bu {
		return -1
	}
	if au > bu {
		return 1
	}
	return 0
}

// MakeRange ：构造一个 RichRange
func MakeRange(begin, end uint16, tex Texture, cfg uint32) RichRange {
	var rr RichRange
	rr.Range = Range{Begin: begin, End: end}
	rr.Accessory.Texture = tex
	rr.Accessory.Config = cfg
	return rr
}

// ====================== interval tree（红黑树）操作 ========================

// RichRangeTree ：一棵树的结构体操作封装
type RichRangeTree struct {
	root int32
	pool *NodePool
}

func NewRichRangeTree(pool *NodePool) RichRangeTree {
	return RichRangeTree{root: nilIdx, pool: pool}
}

func (t *RichRangeTree) Root() int32                 { return t.root }
func (t *RichRangeTree) SetRoot(r int32)             { t.root = normalizeRoot(r) }
func (t *RichRangeTree) IsEmpty() bool               { return t.root < 0 }
func (t *RichRangeTree) node(i int32) *RichRangeNode { return &t.pool.nodes[i] }

// recomputeMaxEnd：根据孩子重新计算 maxEnd
func (t *RichRangeTree) recomputeMaxEnd(i int32) {
	n := t.node(i)
	me := n.Range.End
	if n.left != nilIdx {
		if l := t.node(n.left).maxEnd; l > me {
			me = l
		}
	}
	if n.right != nilIdx {
		if r := t.node(n.right).maxEnd; r > me {
			me = r
		}
	}
	n.maxEnd = me
}

// fixMaxEndUpward：从某个节点一路向上修正 maxEnd
func (t *RichRangeTree) fixMaxEndUpward(i int32) {
	for steps := 0; i != nilIdx; steps++ {
		if steps > 64 {
			panic("fixMaxEndUpward: parent chain too long (cycle suspected)")
		}
		n := t.node(i)
		p := n.parent
		if p == i {
			panic("fixMaxEndUpward: parent points to self (corrupted tree)")
		}
		t.recomputeMaxEnd(i)
		i = p
	}
}

func (t *RichRangeTree) leftRotate(x int32) {
	nx := t.node(x)
	y := nx.right
	if y == nilIdx {
		return
	}
	ny := t.node(y)

	// x.right = y.left
	nx.right = ny.left
	if ny.left != nilIdx {
		t.node(ny.left).parent = x
	}

	// y.parent = x.parent
	ny.parent = nx.parent
	if nx.parent == nilIdx {
		t.root = y
	} else if x == t.node(nx.parent).left {
		t.node(nx.parent).left = y
	} else {
		t.node(nx.parent).right = y
	}

	// y.left = x
	ny.left = x
	nx.parent = y

	// maxEnd 修正：先 x 再 y
	t.recomputeMaxEnd(x)
	t.recomputeMaxEnd(y)
}

func (t *RichRangeTree) rightRotate(y int32) {
	ny := t.node(y)
	x := ny.left
	if x == nilIdx {
		return
	}
	nx := t.node(x)

	// y.left = x.right
	ny.left = nx.right
	if nx.right != nilIdx {
		t.node(nx.right).parent = y
	}

	// x.parent = y.parent
	nx.parent = ny.parent
	if ny.parent == nilIdx {
		t.root = x
	} else if y == t.node(ny.parent).right {
		t.node(ny.parent).right = x
	} else {
		t.node(ny.parent).left = x
	}

	// x.right = y
	nx.right = y
	ny.parent = x

	// maxEnd 修正：先 y 再 x
	t.recomputeMaxEnd(y)
	t.recomputeMaxEnd(x)
}

func (t *RichRangeTree) insertFixup(z int32) {
	for {
		p := t.node(z).parent
		if p == nilIdx || t.node(p).color == black {
			break
		}
		g := t.node(p).parent
		if g == nilIdx {
			break
		}

		if p == t.node(g).left {
			y := t.node(g).right // uncle
			if y != nilIdx && t.node(y).color == red {
				t.node(p).color = black
				t.node(y).color = black
				t.node(g).color = red
				z = g
				continue
			}
			if z == t.node(p).right {
				z = p
				t.leftRotate(z)
				p = t.node(z).parent
				g = t.node(p).parent
			}
			t.node(p).color = black
			t.node(g).color = red
			t.rightRotate(g)
		} else {
			y := t.node(g).left
			if y != nilIdx && t.node(y).color == red {
				t.node(p).color = black
				t.node(y).color = black
				t.node(g).color = red
				z = g
				continue
			}
			if z == t.node(p).left {
				z = p
				t.rightRotate(z)
				p = t.node(z).parent
				g = t.node(p).parent
			}
			t.node(p).color = black
			t.node(g).color = red
			t.leftRotate(g)
		}
	}
	if t.root != nilIdx {
		t.node(t.root).color = black
	}
}

// Insert ：插入一个区间（overlay）
func (t *RichRangeTree) Insert(rr RichRange) {
	z := t.pool.alloc(rr)

	y := nilIdx
	x := t.root
	for x != nilIdx {
		y = x
		if cmpRichRange(rr, t.node(x).Range) < 0 {
			x = t.node(x).left
		} else {
			x = t.node(x).right
		}
	}
	t.node(z).parent = y
	if y == nilIdx {
		t.root = z
	} else if cmpRichRange(rr, t.node(y).Range) < 0 {
		t.node(y).left = z
	} else {
		t.node(y).right = z
	}

	// 从插入点往上修 maxEnd，再做红黑修复
	t.fixMaxEndUpward(z)
	t.insertFixup(z)
	// 旋转后局部 maxEnd 已更新，但祖先可能变化，再补一遍
	t.fixMaxEndUpward(z)
}

func (t *RichRangeTree) minimum(x int32) int32 {
	for x != nilIdx && t.node(x).left != nilIdx {
		x = t.node(x).left
	}
	return x
}

func (t *RichRangeTree) transplant(u, v int32) {
	pu := t.node(u).parent
	if pu == nilIdx {
		t.root = v
	} else if u == t.node(pu).left {
		t.node(pu).left = v
	} else {
		t.node(pu).right = v
	}
	if v != nilIdx {
		t.node(v).parent = pu
	}
}

// findExact：按完全相等查找节点
func (t *RichRangeTree) findExact(rr RichRange) int32 {
	x := t.root
	for x != nilIdx {
		c := cmpRichRange(rr, t.node(x).Range)
		if c == 0 {
			return x
		}
		if c < 0 {
			x = t.node(x).left
		} else {
			x = t.node(x).right
		}
	}
	return nilIdx
}

func (t *RichRangeTree) deleteFixup(x int32, xParent int32) {
	// 这里使用“nilIdx 视为黑色哨兵”，xParent 由调用处传入。
	for (x != t.root) && (x == nilIdx || t.node(x).color == black) {
		if xParent == nilIdx {
			break
		}
		if x == t.node(xParent).left {
			w := t.node(xParent).right
			if w != nilIdx && t.node(w).color == red {
				t.node(w).color = black
				t.node(xParent).color = red
				t.leftRotate(xParent)
				w = t.node(xParent).right
			}
			// w 为黑
			wl := int32(nilIdx)
			wr := int32(nilIdx)
			if w != nilIdx {
				wl = t.node(w).left
				wr = t.node(w).right
			}
			if (w == nilIdx || (wl == nilIdx || t.node(wl).color == black)) &&
				(w == nilIdx || (wr == nilIdx || t.node(wr).color == black)) {
				if w != nilIdx {
					t.node(w).color = red
				}
				x = xParent
				xParent = t.node(x).parent
			} else {
				if w != nilIdx && (wr == nilIdx || t.node(wr).color == black) {
					if wl != nilIdx {
						t.node(wl).color = black
					}
					t.node(w).color = red
					t.rightRotate(w)
					w = t.node(xParent).right
					if w != nilIdx {
						wr = t.node(w).right
					}
				}
				if w != nilIdx {
					t.node(w).color = t.node(xParent).color
				}
				t.node(xParent).color = black
				if w != nilIdx && wr != nilIdx {
					t.node(wr).color = black
				}
				t.leftRotate(xParent)
				x = t.root
				xParent = nilIdx
			}
		} else {
			w := t.node(xParent).left
			if w != nilIdx && t.node(w).color == red {
				t.node(w).color = black
				t.node(xParent).color = red
				t.rightRotate(xParent)
				w = t.node(xParent).left
			}
			wl := int32(nilIdx)
			wr := int32(nilIdx)
			if w != nilIdx {
				wl = t.node(w).left
				wr = t.node(w).right
			}
			if (w == nilIdx || (wl == nilIdx || t.node(wl).color == black)) &&
				(w == nilIdx || (wr == nilIdx || t.node(wr).color == black)) {
				if w != nilIdx {
					t.node(w).color = red
				}
				x = xParent
				xParent = t.node(x).parent
			} else {
				if w != nilIdx && (wl == nilIdx || t.node(wl).color == black) {
					if wr != nilIdx {
						t.node(wr).color = black
					}
					t.node(w).color = red
					t.leftRotate(w)
					w = t.node(xParent).left
					if w != nilIdx {
						wl = t.node(w).left
					}
				}
				if w != nilIdx {
					t.node(w).color = t.node(xParent).color
				}
				t.node(xParent).color = black
				if w != nilIdx && wl != nilIdx {
					t.node(wl).color = black
				}
				t.rightRotate(xParent)
				x = t.root
				xParent = nilIdx
			}
		}
	}
	if x != nilIdx {
		t.node(x).color = black
	}
}

// DeleteExact ：按完全相等的 RichRange 删除，返回是否删除成功
func (t *RichRangeTree) DeleteExact(rr RichRange) bool {
	z := t.findExact(rr)
	if z == nilIdx {
		return false
	}

	y := z
	yOriginalColor := t.node(y).color
	var x int32
	var xParent int32

	if t.node(z).left == nilIdx {
		x = t.node(z).right
		xParent = t.node(z).parent
		t.transplant(z, t.node(z).right)
	} else if t.node(z).right == nilIdx {
		x = t.node(z).left
		xParent = t.node(z).parent
		t.transplant(z, t.node(z).left)
	} else {
		y = t.minimum(t.node(z).right)
		yOriginalColor = t.node(y).color
		x = t.node(y).right

		if t.node(y).parent == z {
			xParent = y
			if x != nilIdx {
				t.node(x).parent = y
			}
		} else {
			xParent = t.node(y).parent
			t.transplant(y, t.node(y).right)
			t.node(y).right = t.node(z).right
			t.node(t.node(y).right).parent = y
		}
		t.transplant(z, y)
		t.node(y).left = t.node(z).left
		t.node(t.node(y).left).parent = y
		t.node(y).color = t.node(z).color

		// y 的 maxEnd 需要重新算（孩子已换）
		t.recomputeMaxEnd(y)
	}

	// 删除后回收 z
	t.pool.free(z)

	// 修 maxEnd：从 xParent 往上
	t.fixMaxEndUpward(xParent)

	if yOriginalColor == black {
		t.deleteFixup(x, xParent)
		// 旋转后再补一次 maxEnd
		t.fixMaxEndUpward(xParent)
	}
	return true
}

// PointQuery ：对“点高度 h”，遍历所有覆盖 h 的区间（不分配）
// visit 返回 false 可提前停止
func (t *RichRangeTree) PointQuery(h uint16, visit func(rr RichRange) bool) {
	if t.root < 0 {
		return
	}

	var st [64]int32
	top := 0
	st[top] = t.root
	top++

	for top > 0 {
		top--
		x := st[top]
		if x == nilIdx {
			continue
		}
		n := t.node(x)

		// 剪枝：如果子树最大 end <= h，则该子树所有区间都不可能包含 h（End exclusive）
		if n.maxEnd <= h {
			continue
		}

		// 如果 h < Begin，则右子树 Begin >= 当前 Begin > h，不可能包含 h，只看左子树
		if h < n.Range.Begin {
			// push 前剪枝：child.maxEnd<=h 的子树没必要入栈
			if n.left != nilIdx && t.node(n.left).maxEnd > h {
				st[top] = n.left
				top++
			}
			continue
		}

		// h >= Begin：左右都有可能
		// push 前剪枝：child.maxEnd<=h 的子树没必要入栈
		if n.right != nilIdx && t.node(n.right).maxEnd > h {
			st[top] = n.right
			top++
		}
		if n.left != nilIdx && t.node(n.left).maxEnd > h {
			st[top] = n.left
			top++
		}

		if n.Range.Contains(h) {
			if !visit(n.Range) {
				return
			}
		}
	}
}

// 遍历树里所有 rr（中序，不分配）
func (t RichRangeTree) ForeachAll(visit func(rr RichRange) bool) {
	if t.root < 0 {
		return
	}
	// in-order stack
	var st [64]int32
	top := 0
	cur := t.root

	for cur != nilIdx || top > 0 {
		for cur != nilIdx {
			st[top] = cur
			top++
			cur = t.node(cur).left
		}
		top--
		x := st[top]
		rr := t.node(x).Range
		if !visit(rr) {
			return
		}
		cur = t.node(x).right
	}
}

// RangeQuery ：遍历所有与 [qBegin,qEnd) 有交集的 rr（不分配）
// visit 返回 false 早停
//
// 核心剪枝规则：
// 1) if n.maxEnd <= qBegin：整棵子树所有 End 都 <= qBegin，不可能相交 -> 砍掉
// 2) if n.Begin >= qEnd：右子树 Begin >= n.Begin >= qEnd，不可能满足 Begin < qEnd -> 不下探右子树，只看左
//
// 相交条件：rr.End > qBegin && rr.Begin < qEnd
func (t *RichRangeTree) RangeQuery(q Range, visit func(rr RichRange) bool) {
	if t.root < 0 || q.End-q.Begin == 0 {
		return
	}
	qb, qe := q.Begin, q.End

	var st [64]int32
	top := 0
	st[top] = t.root
	top++

	for top > 0 {
		top--
		x := st[top]
		if x == nilIdx {
			continue
		}
		n := t.node(x)

		// 剪枝1：子树所有 end <= qb，则不可能相交
		if n.maxEnd <= qb {
			continue
		}

		// 剪枝2：若当前节点 Begin >= qe，则右子树 Begin >= 当前 Begin >= qe
		// 右子树不可能满足 rr.Begin < qe => 不下探右子树
		if n.Range.Begin >= qe {
			// push 前剪枝：left.maxEnd<=qb 的子树没必要入栈
			if n.left != nilIdx && t.node(n.left).maxEnd > qb {
				st[top] = n.left
				top++
			}
			continue
		}

		// 当前节点是否相交
		rr := n.Range
		if rr.End > qb && rr.Begin < qe {
			if !visit(rr) {
				return
			}
		}

		// 继续左右（DFS），但 push 前剪枝：child.maxEnd<=qb 的子树没必要入栈
		if n.right != nilIdx && t.node(n.right).maxEnd > qb {
			st[top] = n.right
			top++
		}
		if n.left != nilIdx && t.node(n.left).maxEnd > qb {
			st[top] = n.left
			top++
		}
	}
}

// RangeQueryInOrder ：按 Begin 全局有序（in-order）遍历所有与 q 相交的 RichRange。
// - 输出顺序严格遵循 cmpRichRange（Begin,End,Accessory）排序。
// - 带剪枝：
//  1. 子树 maxEnd <= qBegin => 子树不可能与 q 相交
//  2. 由于 in-order，遇到 rr.Begin >= qEnd 可直接整体结束（后续 begin 只会更大）
//
// visit 返回 false 时提前结束。
func (t *RichRangeTree) RangeQueryInOrder(q Range, visit func(rr RichRange) bool) {
	if q.End-q.Begin == 0 || t.root < 0 {
		return
	}
	qb, qe := q.Begin, q.End

	// 根子树都不可能相交
	if t.node(t.root).maxEnd <= qb {
		return
	}

	var st [64]int32
	top := 0

	// pushLeft：沿着“可能相交”的左链入栈（带 maxEnd 剪枝）
	pushLeft := func(x int32) {
		for x != nilIdx {
			n := t.node(x)

			// 这棵子树整体 maxEnd 都 <= qb => 整棵子树都不可能与 [qb,qe) 相交
			if n.maxEnd <= qb {
				return
			}

			// 先把当前节点入栈（表示：左子树处理完后要处理它）
			st[top] = x
			top++

			// 是否需要继续向左走？
			// 只有当 left 子树存在且 left.maxEnd > qb 才可能相交，否则左子树可整体跳过
			if n.left != nilIdx && t.node(n.left).maxEnd > qb {
				x = n.left
				continue
			}
			return
		}
	}

	pushLeft(t.root)

	for top > 0 {
		// pop
		top--
		x := st[top]
		n := t.node(x)
		rr := n.Range

		b := rr.Begin
		if b >= qe {
			// in-order：后续 begin 只会更大，直接结束
			return
		}

		// 相交判定：End > qb && Begin < qe
		if rr.End > qb && b < qe {
			if !visit(rr) {
				return
			}
		}

		// 进入右子树（右子树的 begin >= 当前 begin）
		// 右子树整体 maxEnd <= qb 时也可跳过
		if b < qe && n.right != nilIdx && t.node(n.right).maxEnd > qb {
			pushLeft(n.right)
		}
	}
}

// FindMaxEndLEFull ：在树中找“最大 End <= h”的区间（若 End 相同取 Begin 更大）。
// - 返回的是完整 RichRange（包含 Accessory.Config），不会丢信息。
// - accept==nil 表示全接受；否则只考虑 accept(rr)==true 的节点。
// - bestEndIn/bestBeginIn/bestIn 用于“跨结构累积 best”（比如 terrain->LP->HP），与现有 FindMaxEndLE 一致。
// - 依赖 maxEnd 剪枝，仍然是“很少访问节点”的版本。
func (t *RichRangeTree) FindMaxEndLEFull(h uint16, bestEndIn uint16, bestBeginIn uint16, bestIn RichRange, accept func(rr RichRange) bool) (bestEnd uint16, bestBegin uint16, best RichRange) {

	bestEnd, bestBegin, best = bestEndIn, bestBeginIn, bestIn
	if t.root < 0 {
		return
	}
	if accept == nil {
		accept = func(rr RichRange) bool { return true }
	}

	var st [64]int32
	top := 0
	st[top] = t.root
	top++

	for top > 0 {
		top--
		x := st[top]
		if x == nilIdx {
			continue
		}
		n := t.node(x)

		// 关键修复：
		// 不能用 <= bestEnd 剪枝，因为 maxEnd == bestEnd 的子树仍可能存在更大 begin 的 tie-break 候选
		if n.maxEnd < bestEnd {
			continue
		}

		b := n.Range.Begin

		// begin > h => 该节点以及右子树都不可能出现 end<=h（因为 end > begin > h）
		if b > h {
			if n.left != nilIdx && t.node(n.left).maxEnd >= bestEnd {
				st[top] = n.left
				top++
			}
			continue
		}

		rr := n.Range
		if rr.End <= h && accept(rr) {
			e := rr.End
			if e > bestEnd || (e == bestEnd && b > bestBegin) {
				bestEnd = e
				bestBegin = b
				best = rr
				// 注意：即使 bestEnd==h，也不能直接 return，
				// 因为还可能存在 end==h 且 begin 更大的候选（tie-break）
			}
		}

		// DFS：为了更快逼近更大 begin，优先右子树（LIFO：先 push left，再 push right）
		if n.left != nilIdx && t.node(n.left).maxEnd >= bestEnd {
			st[top] = n.left
			top++
		}
		if n.right != nilIdx && t.node(n.right).maxEnd >= bestEnd {
			st[top] = n.right
			top++
		}
	}

	return
}

// OrTexturesAtEndLE : 在树中把所有满足 (rr.End==end) 且 accept(rr)==true 的 rr.Texture OR 起来。
// 额外约束：由于调用处 end<=hLimit，这里用 hLimit 做 begin 剪枝（begin>hLimit 的子树不可能有 end<=hLimit）。
func (t *RichRangeTree) OrTexturesAtEndLE(end uint16, hLimit uint16, accept func(rr RichRange) bool) (tex Texture) {
	if t.root < 0 || end == 0 {
		// 旧语义：0 以下是隐式地面（MaterialBase）
		fake := RichRange{}
		fake.Accessory.Texture = TextureMaterBase
		if accept != nil && !accept(fake) { // 会正确处理 ignore 和 filters
			return 0
		}
		return TextureMaterBase
	}
	if accept == nil {
		accept = func(rr RichRange) bool { return true }
	}

	var st [64]int32
	top := 0
	st[top] = t.root
	top++

	for top > 0 {
		top--
		x := st[top]
		if x == nilIdx {
			continue
		}
		n := t.node(x)

		// 剪枝1：子树最大 end 都 < 目标 end，不可能有 End==end
		if n.maxEnd < end {
			continue
		}

		b := n.Range.Begin

		// 剪枝2：begin > hLimit => 该节点及右子树 begin 更大，且 end>begin>hLimit，不可能出现 End==end<=hLimit
		if b > hLimit {
			if n.left != nilIdx && t.node(n.left).maxEnd >= end {
				st[top] = n.left
				top++
			}
			continue
		}

		rr := n.Range
		if rr.End == end && accept(rr) {
			tex |= rr.Accessory.Texture
		}

		// 继续下探：只要子树 maxEnd >= end 才可能含 End==end
		if n.left != nilIdx && t.node(n.left).maxEnd >= end {
			st[top] = n.left
			top++
		}
		if n.right != nilIdx && t.node(n.right).maxEnd >= end {
			st[top] = n.right
			top++
		}
	}
	return tex
}

// FreeAll : 释放整棵树的所有节点，回收到 NodePool free list，并把 root 置空。
// 注意：这是“结构性释放”（NodePool 变胖问题会缓解），不是 Go heap free。
func (t *RichRangeTree) FreeAll() {
	if t.root < 0 {
		return
	}

	// 释放时必须先取出左右孩子索引，再 free 当前节点（因为 free 会改写 left 用作 nextFree）
	st := make([]int32, 0, 64)
	st = append(st, t.root)

	for len(st) > 0 {
		x := st[len(st)-1]
		st = st[:len(st)-1]
		if x == nilIdx {
			continue
		}
		n := t.node(x)
		l, r := n.left, n.right
		if l != nilIdx {
			st = append(st, l)
		}
		if r != nilIdx {
			st = append(st, r)
		}
		t.pool.free(x)
	}

	t.root = nilIdx
}
