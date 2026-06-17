package guard

import "strings"

func inspectFind(tokens []string, root string) GuardResult {
	baseEnd := len(tokens)
	for i, t := range tokens[1:] {
		if strings.HasPrefix(t, "-") {
			baseEnd = i + 1
			break
		}
	}
	for _, p := range tokens[1:baseEnd] {
		if looksPath(p) {
			if res := checkPathToken(p, root); res.Risk != RiskSafe {
				return res
			}
		}
	}
	for i := 1; i < len(tokens); i++ {
		switch tokens[i] {
		case "-delete":
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "find uses -delete and may remove files"}
		case "-exec", "-execdir":
			j := i + 1
			for j < len(tokens) && tokens[j] != ";" && tokens[j] != "\\;" {
				j++
			}
			if j == i+1 {
				return GuardResult{Risk: RiskNeedsConfirm, Reason: "find exec has no visible child command"}
			}
			return inspectCommand(stripFindPlaceholders(tokens[i+1:j]), root)
		}
	}
	return GuardResult{Risk: RiskSafe, Reason: "find read-only"}
}

func stripFindPlaceholders(tokens []string) []string {
	out := tokens[:0]
	for _, t := range tokens {
		if t != "{}" {
			out = append(out, t)
		}
	}
	return out
}
