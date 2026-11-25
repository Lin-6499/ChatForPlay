package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var dicePattern = regexp.MustCompile(`^([1-9][0-9]?)d([1-9][0-9]{0,3})(kh1|kl1)?([+-][0-9]{1,5})?$`)

type diceResult struct {
	Formula  string `json:"formula"`
	Dice     []int  `json:"dice"`
	Kept     []int  `json:"kept"`
	Modifier int    `json:"modifier"`
	Total    int    `json:"total"`
}

func rollDice(formula string) (diceResult, error) {
	result := diceResult{Formula: strings.ToLower(strings.ReplaceAll(strings.TrimSpace(formula), " ", ""))}
	m := dicePattern.FindStringSubmatch(result.Formula)
	if m == nil {
		return result, gameFail(400, "骰式示例：1d20+5、2d6+3、2d20kh1+4（优势）或 2d20kl1（劣势）")
	}
	n, _ := strconv.Atoi(m[1])
	sides, _ := strconv.Atoi(m[2])
	modifier, _ := strconv.Atoi(m[4])
	if n > 50 || sides < 2 || sides > 1000 || modifier < -10000 || modifier > 10000 {
		return result, gameFail(400, "每次最多 50 颗骰子，面数 2–1000，修正值不超过 ±10000")
	}
	result.Modifier = modifier
	for range n {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
		if err != nil {
			return result, fmt.Errorf("随机数不可用")
		}
		result.Dice = append(result.Dice, int(value.Int64())+1)
	}
	result.Kept = append([]int{}, result.Dice...)
	if m[3] != "" {
		best := result.Dice[0]
		for _, value := range result.Dice {
			if (m[3] == "kh1" && value > best) || (m[3] == "kl1" && value < best) {
				best = value
			}
		}
		result.Kept = []int{best}
	}
	result.Total = modifier
	for _, value := range result.Kept {
		result.Total += value
	}
	return result, nil
}
