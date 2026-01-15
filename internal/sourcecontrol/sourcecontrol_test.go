package sourcecontrol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Commit Message Editing Tests
// =============================================================================

func TestAppendToCommitMsg_EmptyMessage(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	p.AppendToCommitMsg('H')
	assert.Equal(t, "H", p.CommitMsg)
	assert.Equal(t, 1, p.CommitCursor)

	p.AppendToCommitMsg('i')
	assert.Equal(t, "Hi", p.CommitMsg)
	assert.Equal(t, 2, p.CommitCursor)
}

func TestAppendToCommitMsg_InsertAtCursor(t *testing.T) {
	p := &Panel{CommitMsg: "Hllo", CommitCursor: 1}

	p.AppendToCommitMsg('e')
	assert.Equal(t, "Hello", p.CommitMsg)
	assert.Equal(t, 2, p.CommitCursor)
}

func TestAppendToCommitMsg_InsertAtEnd(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 5}

	p.AppendToCommitMsg('!')
	assert.Equal(t, "Hello!", p.CommitMsg)
	assert.Equal(t, 6, p.CommitCursor)
}

func TestBackspaceCommitMsg_DeletesBeforeCursor(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 5}

	p.BackspaceCommitMsg()
	assert.Equal(t, "Hell", p.CommitMsg)
	assert.Equal(t, 4, p.CommitCursor)
}

func TestBackspaceCommitMsg_MiddleOfString(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 3}

	p.BackspaceCommitMsg()
	assert.Equal(t, "Helo", p.CommitMsg)
	assert.Equal(t, 2, p.CommitCursor)
}

func TestBackspaceCommitMsg_AtStart_NoOp(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 0}

	p.BackspaceCommitMsg()
	assert.Equal(t, "Hello", p.CommitMsg)
	assert.Equal(t, 0, p.CommitCursor)
}

func TestDeleteCommitMsg_DeletesAtCursor(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 0}

	p.DeleteCommitMsg()
	assert.Equal(t, "ello", p.CommitMsg)
	assert.Equal(t, 0, p.CommitCursor)
}

func TestDeleteCommitMsg_MiddleOfString(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 2}

	p.DeleteCommitMsg()
	assert.Equal(t, "Helo", p.CommitMsg)
	assert.Equal(t, 2, p.CommitCursor)
}

func TestDeleteCommitMsg_AtEnd_NoOp(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 5}

	p.DeleteCommitMsg()
	assert.Equal(t, "Hello", p.CommitMsg)
	assert.Equal(t, 5, p.CommitCursor)
}

func TestClearCommitMsg(t *testing.T) {
	p := &Panel{CommitMsg: "Some commit message", CommitCursor: 10}

	p.ClearCommitMsg()
	assert.Equal(t, "", p.CommitMsg)
	assert.Equal(t, 0, p.CommitCursor)
}

func TestMoveCursorLeft(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 3}

	p.MoveCursorLeft()
	assert.Equal(t, 2, p.CommitCursor)

	p.MoveCursorLeft()
	assert.Equal(t, 1, p.CommitCursor)
}

func TestMoveCursorLeft_AtStart_NoOp(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 0}

	p.MoveCursorLeft()
	assert.Equal(t, 0, p.CommitCursor)
}

func TestMoveCursorRight(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 2}

	p.MoveCursorRight()
	assert.Equal(t, 3, p.CommitCursor)

	p.MoveCursorRight()
	assert.Equal(t, 4, p.CommitCursor)
}

func TestMoveCursorRight_AtEnd_NoOp(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 5}

	p.MoveCursorRight()
	assert.Equal(t, 5, p.CommitCursor)
}

func TestMoveCursorUp_MovesUpByLineWidth(t *testing.T) {
	p := &Panel{CommitMsg: "Line one text here", CommitCursor: 15}

	p.MoveCursorUp(10)
	assert.Equal(t, 5, p.CommitCursor)
}

func TestMoveCursorUp_ClampsToZero(t *testing.T) {
	p := &Panel{CommitMsg: "Short", CommitCursor: 3}

	p.MoveCursorUp(10)
	assert.Equal(t, 0, p.CommitCursor)
}

func TestMoveCursorUp_FallbackLineWidth(t *testing.T) {
	p := &Panel{CommitMsg: "A long message that spans multiple lines", CommitCursor: 45}

	// lineWidth <= 0 should use fallback of 40
	p.MoveCursorUp(0)
	assert.Equal(t, 5, p.CommitCursor)
}

func TestMoveCursorDown_MovesDownByLineWidth(t *testing.T) {
	p := &Panel{CommitMsg: "A message with enough length", CommitCursor: 5}

	p.MoveCursorDown(10)
	assert.Equal(t, 15, p.CommitCursor)
}

func TestMoveCursorDown_ClampsToLength(t *testing.T) {
	p := &Panel{CommitMsg: "Short", CommitCursor: 3}

	p.MoveCursorDown(10)
	assert.Equal(t, 5, p.CommitCursor) // len("Short") = 5
}

func TestMoveCursorDown_FallbackLineWidth(t *testing.T) {
	msg := "A very long message that definitely spans more than forty characters in total"
	p := &Panel{CommitMsg: msg, CommitCursor: 5}

	// lineWidth <= 0 should use fallback of 40
	p.MoveCursorDown(0)
	assert.Equal(t, 45, p.CommitCursor)
}

func TestInsertNewline(t *testing.T) {
	p := &Panel{CommitMsg: "Line1Line2", CommitCursor: 5}

	p.InsertNewline()
	assert.Equal(t, "Line1\nLine2", p.CommitMsg)
	assert.Equal(t, 6, p.CommitCursor)
}

func TestInsertNewline_AtStart(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 0}

	p.InsertNewline()
	assert.Equal(t, "\nHello", p.CommitMsg)
	assert.Equal(t, 1, p.CommitCursor)
}

func TestInsertNewline_AtEnd(t *testing.T) {
	p := &Panel{CommitMsg: "Hello", CommitCursor: 5}

	p.InsertNewline()
	assert.Equal(t, "Hello\n", p.CommitMsg)
	assert.Equal(t, 6, p.CommitCursor)
}

// =============================================================================
// PasteToCommitMsg Tests - Whitespace Handling
// =============================================================================

func TestPasteToCommitMsg_SimpleText(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	p.PasteToCommitMsg("Hello world")
	assert.Equal(t, "Hello world", p.CommitMsg)
	assert.Equal(t, 11, p.CommitCursor)
}

func TestPasteToCommitMsg_InsertsAtCursor(t *testing.T) {
	p := &Panel{CommitMsg: "Hello !", CommitCursor: 6}

	p.PasteToCommitMsg("world")
	assert.Equal(t, "Hello world!", p.CommitMsg)
	assert.Equal(t, 11, p.CommitCursor)
}

func TestPasteToCommitMsg_ReplacesNewlinesWithSpaces(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	p.PasteToCommitMsg("Line1\nLine2\nLine3")
	assert.Equal(t, "Line1 Line2 Line3", p.CommitMsg)
}

func TestPasteToCommitMsg_ReplacesCarriageReturns(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	p.PasteToCommitMsg("Line1\r\nLine2\rLine3")
	assert.Equal(t, "Line1 Line2 Line3", p.CommitMsg)
}

func TestPasteToCommitMsg_CollapsesMultipleSpaces(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	// Simulates terminal line padding
	p.PasteToCommitMsg("Word1    Word2      Word3")
	assert.Equal(t, "Word1 Word2 Word3", p.CommitMsg)
}

func TestPasteToCommitMsg_TrimsWhitespace(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	p.PasteToCommitMsg("   Hello world   ")
	assert.Equal(t, "Hello world", p.CommitMsg)
}

func TestPasteToCommitMsg_ComplexWhitespace(t *testing.T) {
	p := &Panel{CommitMsg: "", CommitCursor: 0}

	// Simulates pasting from terminal with line breaks and padding
	p.PasteToCommitMsg("  Line1  \n  Line2  \r\n  Line3  ")
	assert.Equal(t, "Line1 Line2 Line3", p.CommitMsg)
}

// =============================================================================
// Section Navigation Tests - MoveUp
// =============================================================================

func TestMoveUp_FromPullBtn_ToPushBtn(t *testing.T) {
	p := &Panel{Section: SectionPullBtn}

	p.MoveUp()
	assert.Equal(t, SectionPushBtn, p.Section)
}

func TestMoveUp_FromPushBtn_ToCommitBtn(t *testing.T) {
	p := &Panel{Section: SectionPushBtn}

	p.MoveUp()
	assert.Equal(t, SectionCommitBtn, p.Section)
}

func TestMoveUp_FromCommitBtn_ToCommitInput(t *testing.T) {
	p := &Panel{Section: SectionCommitBtn}

	p.MoveUp()
	assert.Equal(t, SectionCommitInput, p.Section)
}

func TestMoveUp_FromCommitInput_ToStaged(t *testing.T) {
	p := &Panel{
		Section:     SectionCommitInput,
		StagedFiles: []FileStatus{{Path: "file1.go"}, {Path: "file2.go"}},
	}

	p.MoveUp()
	assert.Equal(t, SectionStaged, p.Section)
	assert.Equal(t, 1, p.Selected) // Last item in staged
}

func TestMoveUp_FromCommitInput_ToStaged_EmptyFiles(t *testing.T) {
	p := &Panel{
		Section:     SectionCommitInput,
		StagedFiles: []FileStatus{},
	}

	p.MoveUp()
	assert.Equal(t, SectionStaged, p.Section)
	assert.Equal(t, 0, p.Selected) // Clamps to 0
}

func TestMoveUp_WithinStaged(t *testing.T) {
	p := &Panel{
		Section:     SectionStaged,
		Selected:    2,
		StagedFiles: []FileStatus{{}, {}, {}},
	}

	p.MoveUp()
	assert.Equal(t, SectionStaged, p.Section)
	assert.Equal(t, 1, p.Selected)
}

func TestMoveUp_FromStaged_ToUnstaged(t *testing.T) {
	p := &Panel{
		Section:       SectionStaged,
		Selected:      0,
		UnstagedFiles: []FileStatus{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}},
	}

	p.MoveUp()
	assert.Equal(t, SectionUnstaged, p.Section)
	assert.Equal(t, 2, p.Selected) // Last item in unstaged
}

func TestMoveUp_FromStaged_ToUnstaged_EmptyFiles(t *testing.T) {
	p := &Panel{
		Section:       SectionStaged,
		Selected:      0,
		UnstagedFiles: []FileStatus{},
	}

	p.MoveUp()
	assert.Equal(t, SectionUnstaged, p.Section)
	assert.Equal(t, 0, p.Selected) // Clamps to 0
}

func TestMoveUp_WithinUnstaged(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      2,
		UnstagedFiles: []FileStatus{{}, {}, {}},
	}

	p.MoveUp()
	assert.Equal(t, SectionUnstaged, p.Section)
	assert.Equal(t, 1, p.Selected)
}

func TestMoveUp_AtTopOfUnstaged_StaysAtTop(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      0,
		UnstagedFiles: []FileStatus{{}, {}},
	}

	p.MoveUp()
	assert.Equal(t, SectionUnstaged, p.Section)
	assert.Equal(t, 0, p.Selected) // Can't go higher
}

// =============================================================================
// Section Navigation Tests - MoveDown
// =============================================================================

func TestMoveDown_WithinUnstaged(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      0,
		UnstagedFiles: []FileStatus{{}, {}, {}},
	}

	p.MoveDown()
	assert.Equal(t, SectionUnstaged, p.Section)
	assert.Equal(t, 1, p.Selected)
}

func TestMoveDown_FromUnstaged_ToStaged(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      2,
		UnstagedFiles: []FileStatus{{}, {}, {}},
	}

	p.MoveDown()
	assert.Equal(t, SectionStaged, p.Section)
	assert.Equal(t, 0, p.Selected)
}

func TestMoveDown_WithinStaged(t *testing.T) {
	p := &Panel{
		Section:     SectionStaged,
		Selected:    0,
		StagedFiles: []FileStatus{{}, {}, {}},
	}

	p.MoveDown()
	assert.Equal(t, SectionStaged, p.Section)
	assert.Equal(t, 1, p.Selected)
}

func TestMoveDown_FromStaged_ToCommitInput(t *testing.T) {
	p := &Panel{
		Section:     SectionStaged,
		Selected:    2,
		StagedFiles: []FileStatus{{}, {}, {}},
	}

	p.MoveDown()
	assert.Equal(t, SectionCommitInput, p.Section)
}

func TestMoveDown_FromCommitInput_ToCommitBtn(t *testing.T) {
	p := &Panel{Section: SectionCommitInput}

	p.MoveDown()
	assert.Equal(t, SectionCommitBtn, p.Section)
}

func TestMoveDown_FromCommitBtn_ToPushBtn(t *testing.T) {
	p := &Panel{Section: SectionCommitBtn}

	p.MoveDown()
	assert.Equal(t, SectionPushBtn, p.Section)
}

func TestMoveDown_FromPushBtn_ToPullBtn(t *testing.T) {
	p := &Panel{Section: SectionPushBtn}

	p.MoveDown()
	assert.Equal(t, SectionPullBtn, p.Section)
}

func TestMoveDown_AtPullBtn_StaysAtPullBtn(t *testing.T) {
	p := &Panel{Section: SectionPullBtn}

	p.MoveDown()
	assert.Equal(t, SectionPullBtn, p.Section) // Can't go lower
}

// =============================================================================
// Section Navigation Tests - NextSection (Tab)
// =============================================================================

func TestNextSection_CyclesThroughAllSections(t *testing.T) {
	p := &Panel{Section: SectionUnstaged, Selected: 5, TopLine: 3}

	// Unstaged -> Staged
	p.NextSection()
	assert.Equal(t, SectionStaged, p.Section)
	assert.Equal(t, 0, p.Selected) // Reset
	assert.Equal(t, 0, p.TopLine)  // Reset

	// Staged -> CommitInput
	p.NextSection()
	assert.Equal(t, SectionCommitInput, p.Section)

	// CommitInput -> CommitBtn
	p.NextSection()
	assert.Equal(t, SectionCommitBtn, p.Section)

	// CommitBtn -> PushBtn
	p.NextSection()
	assert.Equal(t, SectionPushBtn, p.Section)

	// PushBtn -> PullBtn
	p.NextSection()
	assert.Equal(t, SectionPullBtn, p.Section)

	// PullBtn -> Unstaged (wraps around)
	p.NextSection()
	assert.Equal(t, SectionUnstaged, p.Section)
}

func TestNextSection_ResetsSelectionAndTopLine(t *testing.T) {
	p := &Panel{
		Section:  SectionUnstaged,
		Selected: 10,
		TopLine:  5,
	}

	p.NextSection()

	assert.Equal(t, 0, p.Selected)
	assert.Equal(t, 0, p.TopLine)
}

// =============================================================================
// State Predicate Tests
// =============================================================================

func TestCanCommit_TrueWhenStagedFiles(t *testing.T) {
	p := &Panel{
		StagedFiles: []FileStatus{{Path: "file.go"}},
	}

	assert.True(t, p.CanCommit())
}

func TestCanCommit_FalseWhenNoStagedFiles(t *testing.T) {
	p := &Panel{
		StagedFiles: []FileStatus{},
	}

	assert.False(t, p.CanCommit())
}

func TestCanPush_TrueWhenAhead(t *testing.T) {
	p := &Panel{AheadCount: 3}

	assert.True(t, p.CanPush())
}

func TestCanPush_FalseWhenNotAhead(t *testing.T) {
	p := &Panel{AheadCount: 0}

	assert.False(t, p.CanPush())
}

func TestCanPull_TrueWhenBehind(t *testing.T) {
	p := &Panel{BehindCount: 2}

	assert.True(t, p.CanPull())
}

func TestCanPull_FalseWhenNotBehind(t *testing.T) {
	p := &Panel{BehindCount: 0}

	assert.False(t, p.CanPull())
}

// =============================================================================
// GetSelectedFile Tests
// =============================================================================

func TestGetSelectedFile_ReturnsCorrectFile_Unstaged(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      1,
		UnstagedFiles: []FileStatus{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}},
	}

	file := p.GetSelectedFile()
	assert.NotNil(t, file)
	assert.Equal(t, "b.go", file.Path)
}

func TestGetSelectedFile_ReturnsCorrectFile_Staged(t *testing.T) {
	p := &Panel{
		Section:     SectionStaged,
		Selected:    0,
		StagedFiles: []FileStatus{{Path: "staged.go"}},
	}

	file := p.GetSelectedFile()
	assert.NotNil(t, file)
	assert.Equal(t, "staged.go", file.Path)
}

func TestGetSelectedFile_ReturnsNil_WhenOutOfBounds(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      10, // Out of bounds
		UnstagedFiles: []FileStatus{{Path: "a.go"}},
	}

	file := p.GetSelectedFile()
	assert.Nil(t, file)
}

func TestGetSelectedFile_ReturnsNil_WhenNegativeIndex(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		Selected:      -1,
		UnstagedFiles: []FileStatus{{Path: "a.go"}},
	}

	file := p.GetSelectedFile()
	assert.Nil(t, file)
}

func TestGetSelectedFile_ReturnsNil_ForNonFileSections(t *testing.T) {
	p := &Panel{
		Section: SectionCommitInput,
	}

	file := p.GetSelectedFile()
	assert.Nil(t, file)
}

// =============================================================================
// GetCurrentSectionFiles Tests
// =============================================================================

func TestGetCurrentSectionFiles_Unstaged(t *testing.T) {
	p := &Panel{
		Section:       SectionUnstaged,
		UnstagedFiles: []FileStatus{{Path: "a.go"}, {Path: "b.go"}},
		StagedFiles:   []FileStatus{{Path: "c.go"}},
	}

	files := p.GetCurrentSectionFiles()
	assert.Len(t, files, 2)
	assert.Equal(t, "a.go", files[0].Path)
}

func TestGetCurrentSectionFiles_Staged(t *testing.T) {
	p := &Panel{
		Section:       SectionStaged,
		UnstagedFiles: []FileStatus{{Path: "a.go"}},
		StagedFiles:   []FileStatus{{Path: "b.go"}, {Path: "c.go"}},
	}

	files := p.GetCurrentSectionFiles()
	assert.Len(t, files, 2)
	assert.Equal(t, "b.go", files[0].Path)
}

func TestGetCurrentSectionFiles_OtherSections_ReturnsNil(t *testing.T) {
	sections := []Section{SectionCommitInput, SectionCommitBtn, SectionPushBtn, SectionPullBtn}

	for _, section := range sections {
		p := &Panel{
			Section:       section,
			UnstagedFiles: []FileStatus{{Path: "a.go"}},
			StagedFiles:   []FileStatus{{Path: "b.go"}},
		}

		files := p.GetCurrentSectionFiles()
		assert.Nil(t, files, "Section %d should return nil", section)
	}
}
