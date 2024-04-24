package ids

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gammazero/deque"
)

type Solution map[string][]string
type SolutionDistance map[string]int

var Wg sync.WaitGroup
var ReqWg sync.WaitGroup
var WriteLock sync.Mutex
var ReadLock sync.Mutex
var ThreadLock sync.Mutex
var ReadCount uint64
var ThreadCount uint8

var WikiLinkRegex = regexp.MustCompile(`^/wiki/.*`)
var BugRegex = regexp.MustCompile(`.*2024/.*`)

func TitleToUrl(title string) string {
	return ("https://en.wikipedia.org/wiki/" + strings.Join(strings.Split(title, " "), "_"))
}

func GetUntil(p, ms string) string {
	i := strings.Index(ms, p)
	if i == -1 {
		return ""
	}
	return ms[0:i]
}

func GetHyperlinks(url string, NearbyNode *map[string][]string) {
	defer Wg.Done()

	var result []string
	eksis := make(map[string]bool)

	ReqWg.Wait()
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		fmt.Printf("Failed while opening url : %s\n", url)
		fmt.Printf("status code error: %d %s\n\n", res.StatusCode, res.Status)

		if res.StatusCode == 429 {
			ReqWg.Add(1)
			time.Sleep(15 * time.Second)
			ReqWg.Done()
			Wg.Add(1)
			GetHyperlinks(url, NearbyNode)
			return
		}
		return
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Find the review items
	doc.Find("#content a").Each(func(i int, s *goquery.Selection) {

		// Get the hyperlink URL
		href, exist := s.Attr("href")

		if !exist {
			return // Skip if there's no href attribute
		}

		if !WikiLinkRegex.MatchString(href) {
			return
		}

		if BugRegex.MatchString(href) {
			href = GetUntil("2024/", href)
		}

		newNode := "https://en.wikipedia.org" + href
		if !eksis[newNode] && newNode != url {
			// fmt.Println(newNode)
			eksis[newNode] = true
			result = append(result, "https://en.wikipedia.org"+href)
		}
	})
	// fmt.Println(" source: ", url)
	WriteLock.Lock()
	(*NearbyNode)[url] = result
	// fmt.Println(ThreadCount)
	WriteLock.Unlock()
}

func Dls(source string, target string, currentDepth int, MaxDepth int, NearbyNode *map[string][]string, Parent *map[string][]string, NodeVisited *map[string]bool, ChildVisited *map[string]bool, ClosestDist *map[string]int) {
	if currentDepth > 0 {
		(*NodeVisited)[source] = true
		// fmt.Println("curDepth: ", currentDepth, " source: ", source)
		if !(*ChildVisited)[source] {

			// read current nearby node
			ReadLock.Lock()
			ReadCount += 1
			if ReadCount == 1 {
				WriteLock.Lock()
			}
			ReadLock.Unlock()
			currentNearby := (*NearbyNode)[source]
			ReadLock.Lock()
			ReadCount -= 1
			if ReadCount == 0 {
				WriteLock.Unlock()
			}
			ReadLock.Unlock()

			// do gethyperlink for all children
			if currentDepth > 1 {
				ThreadCount = 0
				for _, node := range currentNearby {

					ReadLock.Lock()
					ReadCount += 1
					if ReadCount == 1 {
						WriteLock.Lock()
					}
					ReadLock.Unlock()
					_, nearbyExist := (*NearbyNode)[node]
					ReadLock.Lock()
					ReadCount -= 1
					if ReadCount == 0 {
						WriteLock.Unlock()
					}
					ReadLock.Unlock()

					if !nearbyExist {
						ThreadLock.Lock()
						if ThreadCount >= 250 {
							Wg.Wait()
							time.Sleep(time.Second)
							ThreadCount = 0
						}
						ThreadLock.Unlock()
						Wg.Add(1)
						ThreadCount += 1
						go GetHyperlinks(node, NearbyNode)
					}
				}
				Wg.Wait()
			}

			// fmt.Println(currentNearby)
			for _, node := range currentNearby {
				_, PathExist := (*ClosestDist)[node]
				if !PathExist {
					(*ClosestDist)[node] = (*ClosestDist)[source] + 1
				}
				if (*ClosestDist)[node] >= (*ClosestDist)[source]+1 {
					if (*ClosestDist)[node] > (*ClosestDist)[source]+1 {
						(*ClosestDist)[node] = (*ClosestDist)[source] + 1
						(*Parent)[node] = []string{}
					}
					(*Parent)[node] = append((*Parent)[node], source)
					if target == node {
						(*NodeVisited)[node] = true

					} else if !(*NodeVisited)[node] {
						Dls(node, target, currentDepth-1, MaxDepth, NearbyNode, Parent, NodeVisited, ChildVisited, ClosestDist)
					}
				}
				// fmt.Println("Node: ", node)
				// fmt.Println("pathLength: ", ClosestDist[node])
			}
			(*ChildVisited)[source] = true
		}
		(*NodeVisited)[source] = false
	}
}

func EliminateUnnecessarySolution(source string, target string, Parent Solution, ClosestDist SolutionDistance) (Solution, SolutionDistance) {
	var solution Solution
	var solutionDistance SolutionDistance
	var queue deque.Deque[string]
	var visited map[string]bool
	var currentNode string

	solution = make(Solution)
	solutionDistance = make(SolutionDistance)
	visited = make(map[string]bool)

	queue.PushBack(target)
	for !(queue.Len() == 0) {
		currentNode = queue.PopBack()
		if !visited[currentNode] {
			solution[currentNode] = Parent[currentNode]
			solutionDistance[currentNode] = ClosestDist[currentNode]
			for _, currentNearby := range Parent[currentNode] {
				queue.PushBack(currentNearby)
			}
			visited[currentNode] = true
		}
	}
	return solution, solutionDistance
}

func IdsProccess(source string, target string, MaxDepth int, NearbyNode *map[string][]string) (Solution, SolutionDistance) {
	NodeVisited := make(map[string]bool)
	ChildVisited := make(map[string]bool)
	ClosestDist := make(map[string]int)
	ClosestDist[source] = 0
	Parent := make(map[string][]string) //menunjukkan nilai Parent
	Dls(source, target, MaxDepth, MaxDepth, NearbyNode, &Parent, &NodeVisited, &ChildVisited, &ClosestDist)
	if !NodeVisited[target] && MaxDepth < 10 {
		MaxDepth += 1
		return IdsProccess(source, target, MaxDepth+1, NearbyNode)
	} else {
		return EliminateUnnecessarySolution(source, target, Parent, ClosestDist)
	}
}

func (solution Solution) PrintParent(current string, target string, firstRecursive bool, AlreadyPrinted map[string]bool) {
	if firstRecursive {
		AlreadyPrinted = make(map[string]bool)
		current = TitleToUrl(current)
		target = TitleToUrl(target)
	}
	if (AlreadyPrinted)[current] == false {
		(AlreadyPrinted)[current] = true
		fmt.Println("Node: ", current)
		if current != target {
			fmt.Println("parentnya: ", solution[current])
			for _, node := range solution[current] {
				solution.PrintParent(node, target, false, AlreadyPrinted)
			}
		}
	}
}

func (Parent Solution) PrintPerPath(current string, firstNode string, currentOutput string) {
	if current == firstNode {
		currentOutput = current + ", " + currentOutput
		fmt.Println(currentOutput)
	} else {
		for _, currentParent := range Parent[current] {
			if currentOutput != "" {
				Parent.PrintPerPath(currentParent, firstNode, current+", "+currentOutput)
			} else {
				Parent.PrintPerPath(currentParent, firstNode, current)
			}
		}
	}
}

func GetIdsResult(source string, target string) (Solution, SolutionDistance, string, string) {
	source = TitleToUrl(source)
	target = TitleToUrl(target)
	NearbyNode := make(map[string][]string)
	Wg.Add(1)
	go GetHyperlinks(source, &NearbyNode)
	Wg.Wait()
	solution, solutionDistance := IdsProccess(source, target, 0, &NearbyNode)
	return solution, solutionDistance, source, target
}

// use example
// func main() {
// 	var hasil ids.Solution
// 	source := "Ostrich"
// 	target := "Camel"
// 	hasil, hasilJarak, _, _ := ids.GetIdsResult(source, target)
// 	fmt.Println(hasil)
// 	fmt.Println(hasilJarak)
// }
