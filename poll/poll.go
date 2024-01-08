package poll

import (
	"fmt"

	"github.com/eolso/threadsafe"
	"github.com/olympus-go/eris/utils"
)

type Poll struct {
	uid       string
	prompt    string
	options   []string
	voters    *threadsafe.Map[string, int]
	anonymous bool
}

func NewPoll(prompt string, anonymous bool, options ...string) *Poll {
	var p Poll

	p.uid = utils.ShaSum(prompt)
	p.prompt = prompt
	p.options = options
	p.voters = threadsafe.NewMap[string, int]()
	p.anonymous = anonymous

	return &p
}

func (p *Poll) Vote(name string, option int) {
	if option < 0 || option >= len(p.options) {
		return
	}

	p.voters.Set(name, option)
}

func (p *Poll) String() string {
	// Generate a mapping of options to voters
	optionsMap := make(map[int][]string)
	keys, values := p.voters.Items()

	for i, _ := range keys {
		optionsMap[values[i]] = append(optionsMap[values[i]], keys[i])
	}

	str := fmt.Sprintf("%s\n```", p.prompt)
	for i, option := range p.options {
		var voters []string
		if v, ok := optionsMap[i]; ok {
			voters = v
		}

		str += fmt.Sprintf("%d) %s <%d>", i+1, option, len(voters))
		if !p.anonymous {
			for _, voter := range voters {
				str += fmt.Sprintf(" %s", voter)
			}
		}
		str += fmt.Sprintf("\n")
	}
	str += "```"

	return str
}
