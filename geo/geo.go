package geo

import (
	"context"
	"fmt"
	"net"
	"time"

	"gitee.com/dark.H/gs"
	"github.com/ip2location/ip2location-go/v9"
)

type Geo struct {
	DOMAIN  string
	IP      string
	Country string
	City    string
}

func (g *Geo) Str() gs.Str {
	return gs.Str("(%s/%s)[%s/%s]").F(g.IP, g.DOMAIN, g.Country, g.City)
}

func (g *Geo) InCN() bool {
	return g.Country == "China"
}

func (g *Geo) InUSA() bool {
	return g.Country == "United States of America"
}

func IP2GEO(ip ...string) (points gs.List[*Geo]) {
	path := "/.cache/geodb"
	db, err := ip2location.OpenDB(path)
	if err != nil {

		path = gs.Str("~").ExpandUser().PathJoin(".cache", "geodatabase", "IP2LOCATION-LITE-DB3.BIN").Str()
		db, err = ip2location.OpenDB(path)
		if err != nil {
			fmt.Print(err)
			return
		}
	}
	defer db.Close()

	for _, i := range ip {
		results, err := db.Get_all(i)
		if err != nil {
			return nil
		}
		points = points.Add(&Geo{
			IP:      i,
			City:    results.City,
			Country: results.Country_long,
		})
	}
	return
}

func Host2GEO(hosts ...string) (points gs.List[*Geo]) {
	path := "/.cache/geodb"
	db, err := ip2location.OpenDB(path)
	if err != nil {
		path = gs.Str("~").ExpandUser().PathJoin(".cache", "geodatabase", "IP2LOCATION-LITE-DB3.BIN").Str()
		db, err = ip2location.OpenDB(path)
		if err != nil {
			fmt.Print(err)
			return
		}

	}
	defer db.Close()
	r := &net.Resolver{}

	for _, i := range hosts {
		ct, _ := context.WithTimeout(context.Background(), 700*time.Millisecond)
		addrs, err := r.LookupHost(ct, i)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			results, err := db.Get_all(a)
			if err != nil {
				return nil
			}
			points = points.Add(&Geo{
				DOMAIN:  i,
				IP:      a,
				City:    results.City,
				Country: results.Country_long,
			})
			break
		}
	}
	return
}
