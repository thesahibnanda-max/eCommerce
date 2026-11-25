package httpErrorHandler

import "errors"

func validRules(rules []Rule) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.Err != nil && r.StatusCode != 0 {
			out = append(out, r)
		}
	}
	return out
}

func (eh *errorHandler) match(err error) (Rule, bool) {
	for _, rule := range eh.rules {
		if errors.Is(err, rule.Err) {
			return rule, true
		}
	}
	return Rule{}, false
}
