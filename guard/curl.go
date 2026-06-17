package guard

import "strings"

func inspectCurl(tokens []string) GuardResult {
	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		if t == "--" {
			break
		}
		if !strings.HasPrefix(t, "-") {
			continue
		}
		if curlOptionHasWriteOrMutation(t) {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "curl option may write files or mutate remote state: " + t}
		}
		if (t == "-X" || t == "--request") && i+1 < len(tokens) {
			method := strings.ToUpper(tokens[i+1])
			if method != "GET" && method != "HEAD" {
				return GuardResult{Risk: RiskNeedsConfirm, Reason: "curl request method may mutate remote state: " + method}
			}
		}
		if curlOptionTakesMutatingValue(t) && i+1 < len(tokens) {
			i++
		}
	}
	return GuardResult{Risk: RiskSafe, Reason: "curl read-only"}
}

func curlOptionHasWriteOrMutation(option string) bool {
	if option == "-O" || strings.HasPrefix(option, "-O") || option == "-J" || strings.HasPrefix(option, "-J") {
		return true
	}
	if option == "-o" || strings.HasPrefix(option, "-o") || option == "--output" || strings.HasPrefix(option, "--output=") {
		return true
	}
	for _, prefix := range []string{"--remote-name", "--remote-header-name", "--output-dir", "--upload-file", "--form", "--form-string", "--data", "--data-", "--request-target"} {
		if option == prefix || strings.HasPrefix(option, prefix+"=") {
			return true
		}
	}
	if option == "-T" || strings.HasPrefix(option, "-T") || option == "-F" || strings.HasPrefix(option, "-F") || option == "-d" || strings.HasPrefix(option, "-d") {
		return true
	}
	if option == "-X" || strings.HasPrefix(option, "-X") || option == "--request" || strings.HasPrefix(option, "--request=") {
		method := ""
		if strings.HasPrefix(option, "--request=") {
			method = strings.TrimPrefix(option, "--request=")
		} else if strings.HasPrefix(option, "-X") && len(option) > 2 {
			method = option[2:]
		}
		method = strings.ToUpper(method)
		return method != "" && method != "GET" && method != "HEAD"
	}
	return false
}

func curlOptionTakesMutatingValue(option string) bool {
	return option == "-o" || option == "--output" || option == "--output-dir" || option == "-T" || option == "--upload-file" || option == "-F" || option == "--form" || option == "--form-string" || option == "-d" || strings.HasPrefix(option, "--data") || option == "-X" || option == "--request"
}
