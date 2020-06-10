package core

import (
	"fmt"
	"io"
	"regexp"
	"unicode/utf8"
)

func seqFirst(seq Seq, w io.Writer, indent int) (Seq, int) {
	if !seq.IsEmpty() {
		indent = formatObject(seq.First(), indent, w)
		seq = seq.Rest()
	}
	return seq, indent
}

// TODO: maybe merge it with seqFirstAfterBreak
// or extract common part into a separate function
func seqFirstAfterSpace(seq Seq, w io.Writer, indent int, insideDefRecord bool) (Seq, Object, int) {
	var obj Object
	if !seq.IsEmpty() {
		fmt.Fprint(w, " ")
		obj = seq.First()
		if s, ok := obj.(Seq); ok && !obj.Equals(NIL) {
			if info := obj.GetInfo(); info != nil {
				fmt.Fprint(w, info.prefix)
				indent += utf8.RuneCountInString(info.prefix)
			}
			indent = formatSeqEx(s, w, indent+1, insideDefRecord)
		} else {
			indent = formatObject(obj, indent+1, w)
		}
		seq = seq.Rest()
	}
	return seq, obj, indent
}

func seqFirstAfterBreak(prevObj Object, seq Seq, w io.Writer, indent int, insideDefRecord bool) (Seq, Object, int) {
	var obj Object
	if !seq.IsEmpty() {
		obj = seq.First()
		cnt := newLineCount(prevObj, obj)
		for i := 0; i < cnt; i++ {
			fmt.Fprint(w, "\n")
		}
		writeIndent(w, indent)
		if s, ok := obj.(Seq); ok && !obj.Equals(NIL) {
			if info := obj.GetInfo(); info != nil {
				fmt.Fprint(w, info.prefix)
				indent += utf8.RuneCountInString(info.prefix)
			}
			indent = formatSeqEx(s, w, indent, insideDefRecord)
		} else {
			indent = formatObject(obj, indent, w)
		}
		seq = seq.Rest()
	}
	return seq, obj, indent
}

func formatBindings(v *Vector, w io.Writer, indent int) int {
	fmt.Fprint(w, "[")
	newIndent := indent + 1
	for i := 0; i < v.count; i += 2 {
		newIndent = formatObject(v.at(i), indent+1, w)
		if i+1 < v.count {
			fmt.Fprint(w, " ")
			newIndent = formatObject(v.at(i+1), newIndent+1, w)
		}
		if i+2 < v.count {
			if isNewLine(v.at(i+1), v.at(i+2)) {
				fmt.Fprint(w, "\n")
				writeIndent(w, indent+1)
			} else {
				fmt.Fprint(w, " ")
			}
		}
	}
	fmt.Fprint(w, "]")
	return newIndent + 1
}

func formatVectorVertically(v *Vector, w io.Writer, indent int) int {
	fmt.Fprint(w, "[")
	newIndent := indent + 1
	for i := 0; i < v.count; i++ {
		newIndent = formatObject(v.at(i), indent+1, w)
		if i+1 < v.count {
			fmt.Fprint(w, "\n")
			writeIndent(w, indent+1)
		}
	}
	fmt.Fprint(w, "]")
	return newIndent + 1
}

var defRegex *regexp.Regexp = regexp.MustCompile("def.*")
var ifRegex *regexp.Regexp = regexp.MustCompile("if(-.+)?")
var whenRegex *regexp.Regexp = regexp.MustCompile("when(-.+)?")
var extendRegex *regexp.Regexp = regexp.MustCompile("extend(-.+)?")
var bodyIndentRegexes []*regexp.Regexp = []*regexp.Regexp{
	regexp.MustCompile("^(bound-fn|if|if-not|case|cond|cond->|cond->>|go|condp|when|while|when-not|when-first|do|future)$"),
	regexp.MustCompile("^(comment|doto|locking|proxy|with-[^\\s]*|reify)$"),
	regexp.MustCompile("^(defprotocol|extend|extend-protocol|extend-type|try|catch|finally|let|letfn|binding|loop|for|go-loop)$"),
	regexp.MustCompile("^(doseq|dotimes|when-let|if-let|defstruct|struct-map|defmethod|testing|deftest|context|use-fixtures)$"),
	regexp.MustCompile("^(POST|GET|PUT|DELETE)"),
	regexp.MustCompile("^(handler-case|handle|dotrace|deftrace)$"),
}

func isOneAndBodyExpr(obj Object) bool {
	switch s := obj.(type) {
	case Symbol:
		return defRegex.MatchString(*s.name) ||
			ifRegex.MatchString(*s.name) ||
			whenRegex.MatchString(*s.name) ||
			extendRegex.MatchString(*s.name)
	default:
		return false
	}
}

func isBodyIndent(obj Object) bool {
	switch s := obj.(type) {
	case Symbol:
		for _, re := range bodyIndentRegexes {
			if re.MatchString(*s.name) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func isNewLine(obj, nextObj Object) bool {
	info, nextInfo := obj.GetInfo(), nextObj.GetInfo()
	return !(info == nil || nextInfo == nil || info.endLine == nextInfo.startLine)
}

func newLineCount(obj, nextObj Object) int {
	info, nextInfo := obj.GetInfo(), nextObj.GetInfo()
	if info == nil || nextInfo == nil {
		return 0
	}
	return nextInfo.startLine - info.endLine
}

func formatSeq(seq Seq, w io.Writer, indent int) int {
	return formatSeqEx(seq, w, indent, false)
}

func formatSeqEx(seq Seq, w io.Writer, indent int, formatAsDef bool) int {
	i := indent + 1
	restIndent := indent + 2
	fmt.Fprint(w, "(")
	obj := seq.First()
	prevObj := obj
	seq, i = seqFirst(seq, w, i)
	isDefRecord := false
	if obj.Equals(SYMBOLS.defrecord) ||
		obj.Equals(SYMBOLS.defprotocol) ||
		obj.Equals(SYMBOLS.extendProtocol) ||
		obj.Equals(SYMBOLS.extendType) {
		isDefRecord = true
	}
	if obj.Equals(SYMBOLS.ns) || isOneAndBodyExpr(obj) {
		seq, prevObj, i = seqFirstAfterSpace(seq, w, i, isDefRecord)
	} else if obj.Equals(KEYWORDS.require) || obj.Equals(KEYWORDS._import) {
		obj = seq.First()
		seq, prevObj, _ = seqFirstAfterSpace(seq, w, i, isDefRecord)
		for !seq.IsEmpty() {
			seq, prevObj, _ = seqFirstAfterBreak(obj, seq, w, i+1, isDefRecord)
			obj = prevObj
		}
	} else if obj.Equals(SYMBOLS.fn) || obj.Equals(SYMBOLS.catch) {
		if !seq.IsEmpty() {
			switch seq.First().(type) {
			case *Vector:
				seq, prevObj, i = seqFirstAfterSpace(seq, w, i, isDefRecord)
			default:
				seq, prevObj, i = seqFirstAfterSpace(seq, w, i, isDefRecord)
				seq, prevObj, i = seqFirstAfterSpace(seq, w, i, isDefRecord)
			}
		}
	} else if obj.Equals(SYMBOLS.let) || obj.Equals(SYMBOLS.loop) {
		if v, ok := seq.First().(*Vector); ok {
			fmt.Fprint(w, " ")
			i = formatBindings(v, w, i+1)
			prevObj = seq.First()
			seq = seq.Rest()
		}
	} else if obj.Equals(SYMBOLS.letfn) {
		if v, ok := seq.First().(*Vector); ok {
			fmt.Fprint(w, " ")
			i = formatVectorVertically(v, w, i+1)
			prevObj = seq.First()
			seq = seq.Rest()
		}
	} else if obj.Equals(SYMBOLS.do) || obj.Equals(SYMBOLS.try) || obj.Equals(SYMBOLS.finally) {
	} else if formatAsDef {
	} else if isBodyIndent(obj) {
		restIndent = indent + 2
	} else {
		// Indent function call arguments.
		restIndent = indent + 1
		if !seq.IsEmpty() && !isNewLine(obj, seq.First()) {
			restIndent = i + 1
		}
	}

	for !seq.IsEmpty() {
		nextObj := seq.First()
		if isNewLine(obj, nextObj) {
			seq, prevObj, i = seqFirstAfterBreak(prevObj, seq, w, restIndent, isDefRecord)
		} else {
			seq, prevObj, i = seqFirstAfterSpace(seq, w, i, isDefRecord)
		}
		obj = nextObj
	}

	fmt.Fprint(w, ")")
	return i + 1
}
