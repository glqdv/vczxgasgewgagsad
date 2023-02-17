package router

import (
	"strconv"
	"sync"

	"gitee.com/dark.H/gs"
)

type Rule struct {
	Table       string `json:"table"`
	Chains      string `json:"chains"`
	ID          int    `json:"id"`
	Target      string `json:"target"`
	Prot        string `json:"prot"`
	Opt         string `json:"opt"`
	Comment     string `json:"comment"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Policy      string `json:"policy"`

	// Extend Option
	Interface       string `json:"interface"`
	SourcePort      int    `json:"sport"`
	DestinationPort int    `json:"dport"`

	// REDIRECT Option
	ToPort        int    `json:"--to-ports"`
	ToDestination string `json:"--to-destination"`
}

var (
	iptable_lock = sync.RWMutex{}
	BY_PROC      = 0
	BY_SRC_IP    = 1
	BY_DST_IP    = 2
	BY_DST_PORT  = 4
	BY_SRC_PORT  = 3
)

type BY int
type Iptables gs.List[*Rule]

func NewRule() (rule *Rule) {
	return &Rule{
		Prot:        "any",
		Source:      "0.0.0.0/0",
		Destination: "0.0.0.0/0",
		Target:      "ACCEPT",
		ID:          -1,
	}
}

func LoadTableRules(table string) (ip Iptables) {
	chains := gs.Str("")
	default_action := gs.Str("")
	action_map := gs.Dict[gs.Str]{}
	// ok := false

	Exec("iptables -t %s -L -n  --line-number").F(table).EveryLine(func(lineno int, line gs.Str) {
		line = line.Trim()
		if line == "" {
			return
		}
		fs := line.SplitSpace()

		if line.StartsWith("Chain ") {
			if fs.Len() == 4 {
				chains = fs[1]
				default_action = fs[3]
				if default_action == "references)" {
					default_action = action_map[chains.Str()]
				} else {
					default_action = default_action.Replace(")", "")
				}
			}
		} else if fs.Len() > 5 {
			if fs[0] != "num" {
				if id_int, err := strconv.Atoi(fs[0].Str()); err == nil {
					rule := &Rule{
						ID:          id_int,
						Table:       table,
						Chains:      string(chains),
						Policy:      default_action.String(),
						Target:      fs[1].Str(),
						Prot:        fs[2].Str(),
						Opt:         fs[3].Str(),
						Source:      fs[4].Str(),
						Destination: fs[5].Str(),
					}
					if fs.Len() > 6 {
						rule.Comment = string(fs[6:].Join(" "))
					}
					switch rule.Target {
					case "RETURN", "REDIRECT", "REJECT", "ACCEPT", "DROP", "LOG", "ULOG", "SNAT", "NAT", "MASQUERADE", "MARK":
					default:
						if _, ok := action_map[rule.Target]; !ok {
							action_map[rule.Target] = default_action
						}
					}
					ip = append(ip, rule)
				}
			}
		}
	})
	return
}

func (rule *Rule) IsFromAny(set ...bool) bool {
	if set != nil && set[0] {
		rule.Source = "0.0.0.0/0"
	}
	return rule.Source == "0.0.0.0/0"
}

func (rule *Rule) IsFromAnyPort(set ...int) bool {
	if set != nil && set[0] > -1 && set[0] < 65536 {
		rule.SourcePort = set[0]
	}
	return rule.SourcePort == 0
}

func (rule *Rule) IsToAnyPort(set ...int) bool {
	if set != nil && set[0] > -1 && set[0] < 65536 {
		rule.DestinationPort = set[0]
	}
	return rule.DestinationPort == 0
}

func (rule *Rule) IsToAny(set ...bool) bool {
	if set != nil && set[0] {
		rule.Destination = "0.0.0.0/0"
	}
	return rule.Destination == "0.0.0.0/0"
}

func (rule *Rule) IsAnyProtocol(set ...bool) bool {
	if set != nil && set[0] {
		rule.Prot = "all"
	}
	return rule.Prot == "all"
}

func (rule *Rule) IsAnyInterface(set ...bool) bool {
	if set != nil && set[0] {
		rule.Interface = ""
	}
	return rule.Interface == ""
}

func (rule *Rule) TargetCommand() gs.Str {

	switch rule.Target {
	case "REDIRECT":
		c := gs.Str("")
		if rule.ToPort > 0 {
			c = gs.Str("--to-ports %d").F(rule.ToPort)
		}
		return c
	case "OUTPUT":
		c := gs.Str("")
		if rule.ToDestination != "" {
			c = gs.Str("--to-destication %s").F(rule.ToDestination)
		}
		return c
	}
	return gs.Str(rule.Target)
}

func (rule *Rule) FilterCommand() gs.Str {
	cmds := gs.List[string]{}
	Interface := ""
	if !rule.IsAnyInterface() {
		Interface = "-i " + rule.Interface
	}
	if Interface != "" {
		cmds = cmds.Add(Interface)
	}

	filter_prot := ""
	if !rule.IsAnyProtocol() {
		filter_prot = "-p " + rule.Prot
	}
	if filter_prot != "" {
		cmds = cmds.Add(filter_prot)
	}

	sip := ""
	if !rule.IsFromAny() {
		sip = "-s " + rule.Source
	}
	if sip != "" {
		cmds = cmds.Add(sip)
	}

	sport := ""
	if !rule.IsFromAny() {
		sport = gs.Str("-sport %d").F(rule.SourcePort).Str()
	}
	if sport != "" {
		cmds = cmds.Add(sport)
	}

	dport := ""
	if !rule.IsToAny() {
		dport = gs.Str("-dport %d").F(rule.DestinationPort).Str()
	}
	if dport != "" {
		cmds = cmds.Add(dport)
	}

	return cmds.Join(" ")
}

func (rule *Rule) SetFilter(by BY, val any) *Rule {

	switch val.(type) {
	case string:
		switch by {
		case BY(BY_DST_IP):
			switch val.(type) {
			case string:
				rule.Destination = val.(string)
			}

		case BY(BY_SRC_IP):
			rule.Source = val.(string)
		case BY(BY_PROC):
			rule.Prot = val.(string)
		}
	case int:
		switch by {
		case BY(BY_SRC_PORT):
			rule.SourcePort = val.(int)
		case BY(BY_DST_PORT):
			rule.DestinationPort = val.(int)
		}
	}
	return rule
}

func (rule *Rule) SetTable(table string) *Rule {
	rule.Table = table
	return rule
}

func (rule *Rule) SetChains(chains string) *Rule {
	rule.Chains = chains
	return rule
}

func (rule *Rule) SetPortRedirect(dport int) *Rule {
	// rule.Chains = "PREROUTING"
	rule.ToPort = dport
	rule.Target = "REDIRECT"
	return rule
}

func (rule *Rule) SetIPAndPortRedirect(ip string, dport int) *Rule {
	// rule.Chains = "PREROUTING"
	rule = rule.SetChains("PREROUTING")
	rule.ToDestination = string(gs.Str("%s:%d").F(ip, dport))
	rule.Target = "DNAT"
	return rule
}

func (rule *Rule) AddCommand(toTop ...bool) gs.Str {
	// gs.Str("[%s-]")
	filter_cmd := rule.FilterCommand()
	target_cmd := rule.TargetCommand()
	add := "A"
	if toTop != nil && toTop[0] {
		add = "I"
	}
	if target_cmd != "" {
		return gs.Str("iptable -t %s -%s %s %s -j %s").F(rule.Table, add, rule.Chains, filter_cmd, target_cmd)
	}
	return ""
}

func (rule *Rule) DelCommand() gs.Str {

	return gs.Str("iptables -t %s -D %s %d").F(rule.Table, rule.Chains, rule.ID)
}

func (iptable Iptables) Delete(rule *Rule) Iptables {
	LockArea(func() {
		if rule != nil && rule.ID > -1 && rule.Chains != "" && rule.Table != "" {
			dd := rule.DelCommand()
			Exec(dd)
			iptable = LoadTableRules(rule.Table)

		}
	})
	return iptable
}

func (iptable Iptables) Add(rule *Rule) Iptables {
	LockArea(func() {
		if rule != nil && rule.ID > -1 && rule.Chains != "" && rule.Table != "" {
			Exec(rule.AddCommand())
			iptable = LoadTableRules(rule.Table)

		}
	})
	return iptable
}

func (iptable Iptables) Save(dpath string) Iptables {
	gs.Str(dpath).Dirname().Mkdir()
	Exec("iptables-save > " + gs.Str(dpath))
	return iptable
}

func (iptable Iptables) Restore(dpath string) Iptables {
	if gs.Str(dpath).IsExists() {
		Exec("iptables-restore  " + gs.Str(dpath))
	}
	return iptable
}

func LockArea(d func()) {
	iptable_lock.Lock()
	d()
	iptable_lock.Unlock()
}
