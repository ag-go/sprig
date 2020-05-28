package icons

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var BackIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationArrowBack)
	return icon
}()

var ClearIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentClear)
	return icon
}()

var ReplyIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentReply)
	return icon
}()

var CancelReplyIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationCancel)
	return icon
}()

var SendReplyIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentSend)
	return icon
}()

var CreateConversationIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentCreate)
	return icon
}()

var CopyIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentContentCopy)
	return icon
}()

var PasteIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentContentPaste)
	return icon
}()
