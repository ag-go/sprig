package main

import (
	"image/color"
	"log"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/fields"
)

type ReplyListView struct {
	manager ViewManager

	*Settings
	*ArborState
	*material.Theme

	BackButton   widget.Clickable
	ReplyList    layout.List
	ReplyStates  []ReplyState
	Selected     *fields.QualifiedHash
	Ancestry     []*fields.QualifiedHash
	Descendants  []*fields.QualifiedHash
	Conversation *fields.QualifiedHash
}

var _ View = &ReplyListView{}

func NewReplyListView(settings *Settings, arborState *ArborState, theme *material.Theme) View {
	c := &ReplyListView{
		Settings:   settings,
		ArborState: arborState,
		Theme:      theme,
	}
	return c
}

func (c *ReplyListView) Update(gtx *layout.Context) {
	for i := range c.ReplyStates {
		clickHandler := &c.ReplyStates[i]
		if clickHandler.Clicked(gtx) {
			log.Printf("clicked %s", clickHandler.Reply)
			c.Selected = clickHandler.Reply
			c.Ancestry, _ = c.ArborState.SubscribableStore.AncestryOf(clickHandler.Reply)
			c.Descendants, _ = c.ArborState.SubscribableStore.DescendantsOf(clickHandler.Reply)
			reply, _, _ := c.ArborState.SubscribableStore.Get(clickHandler.Reply)
			c.Conversation = &reply.(*forest.Reply).ConversationID

		}
	}
}

type ReplyState struct {
	widget.Clickable
	Reply *fields.QualifiedHash
}

func (c *ReplyListView) Layout(gtx *layout.Context) {
	gtx.Constraints.Height.Min = gtx.Constraints.Height.Max
	gtx.Constraints.Width.Min = gtx.Constraints.Width.Max

	theme := c.Theme
	c.ReplyList.Axis = layout.Vertical
	stateIndex := 0
	c.ArborState.ReplyList.WithReplies(func(replies []*forest.Reply) {
		c.ReplyList.Layout(gtx, len(replies), func(index int) {
			if stateIndex >= len(c.ReplyStates) {
				c.ReplyStates = append(c.ReplyStates, ReplyState{})
			}
			state := &c.ReplyStates[stateIndex]
			reply := replies[index]
			authorNode, found, err := c.ArborState.SubscribableStore.GetIdentity(&reply.Author)
			if err != nil || !found {
				log.Printf("failed finding author %s for node %s", &reply.Author, reply.ID())
			}
			author := authorNode.(*forest.Identity)
			layout.Stack{}.Layout(gtx,
				layout.Stacked(func() {
					gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
					leftInset := unit.Dp(0)
					background := color.RGBA{R: 175, G: 175, B: 175, A: 255}
					if c.Selected != nil && reply.ID().Equals(c.Selected) {
						leftInset = unit.Dp(20)
						background.R = 255
						background.G = 255
						background.B = 255
					} else {
						found := false
						for _, id := range c.Ancestry {
							if id.Equals(reply.ID()) {
								leftInset = unit.Dp(20)
								background.R = 230
								background.G = 230
								background.B = 230
								found = true
								break
							}
						}
						if !found {
							for _, id := range c.Descendants {
								if id.Equals(reply.ID()) {
									leftInset = unit.Dp(30)
									background.R = 230
									background.G = 230
									background.B = 230
									found = true
									break
								}
							}
						}
						if !found && c.Conversation != nil && !c.Conversation.Equals(fields.NullHash()) {
							if c.Conversation.Equals(&reply.ConversationID) {
								leftInset = unit.Dp(10)
							}
						}
					}
					layout.Stack{}.Layout(gtx,
						layout.Expanded(func() {
							paintOp := paint.ColorOp{Color: color.RGBA{G: 128, B: 128, A: 255}}
							paintOp.Add(gtx.Ops)
							paint.PaintOp{Rect: f32.Rectangle{
								Max: f32.Point{
									X: float32(gtx.Constraints.Width.Max),
									Y: float32(gtx.Constraints.Height.Max),
								},
							}}.Add(gtx.Ops)
						}),
						layout.Stacked(func() {
							layout.Inset{Left: leftInset}.Layout(gtx, func() {
								layout.Stack{}.Layout(gtx,
									layout.Expanded(func() {
										paintOp := paint.ColorOp{Color: background}
										paintOp.Add(gtx.Ops)
										paint.PaintOp{Rect: f32.Rectangle{
											Max: f32.Point{
												X: float32(gtx.Constraints.Width.Max),
												Y: float32(gtx.Constraints.Height.Max),
											},
										}}.Add(gtx.Ops)
									}),
									layout.Stacked(func() {
										layout.UniformInset(unit.Dp(4)).Layout(gtx, func() {
											layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func() {
													gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
													layout.NW.Layout(gtx, func() {
														name := material.Body2(theme, string(author.Name.Blob))
														name.Font.Weight = text.Bold
														name.Layout(gtx)
													})
													layout.NE.Layout(gtx, func() {
														date := material.Body2(theme, reply.Created.Time().Local().Format("2006/01/02 15:04"))
														date.Color.A = 200
														date.TextSize = unit.Dp(12)
														date.Layout(gtx)
													})
												}),
												layout.Rigid(func() {
													material.Body1(theme, string(reply.Content.Blob)).Layout(gtx)
												}),
											)
										})
									}),
								)
							})
						}),
					)
				}),
				layout.Expanded(func() {
					state.Clickable.Layout(gtx)
					state.Reply = reply.ID()
				}),
			)
			stateIndex++
		})
	})
}

func (c *ReplyListView) SetManager(mgr ViewManager) {
	c.manager = mgr
}
