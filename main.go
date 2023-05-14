package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v52/github"
	"golang.org/x/oauth2"
)

type Prompt struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type ChatGPTRequest struct {
	Model    string   `json:"model"`
	Messages []Prompt `json:"messages"`
}

type ChatGPTResponse struct {
	Message string `json:"message"`
}

func GetGitDiffOutput() ([]byte, error) {
	baseBranch := os.Getenv("GITHUB_BASE_REF")

	reviewIgnorePath := ".review-ignore"
	var patterns []string
	if _, err := os.Stat(reviewIgnorePath); !os.IsNotExist(err) {
		content, err := os.ReadFile(reviewIgnorePath)
		if err != nil {
			return nil, err
		}
		patterns = strings.Split(string(content), "\n")
	}

	cmd := exec.Command("git", "diff", "HEAD", baseBranch, "--", ".")
	if len(patterns) > 0 {
		var args []string
		for _, pattern := range patterns {
			if pattern != "" {
				args = append(args, fmt.Sprintf(":!%s", pattern))
			}
		}
		cmd.Args = append(cmd.Args, args...)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func GetChatGptResponse(diff []byte) ([]byte, error) {
	endpoint := "https://api.openai.com/v1/engines/davinci/completions"
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY environment variable must be set")
	}

	chatGPTRequest := ChatGPTRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Prompt{
			{
				Content: fmt.Sprintf("Please review the following code. You are an excellent software engineer.\n```\n%s\n```", string(diff)),
				Role:    "user",
			},
		},
	}
	requestBody, err := json.Marshal(chatGPTRequest)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chatGPTResponse ChatGPTResponse
	err = json.NewDecoder(resp.Body).Decode(&chatGPTResponse)
	if err != nil {
		return nil, err
	}

	return []byte(chatGPTResponse.Message), nil
}

func SplitRepositoryName(repo string) (string, string, error) {
	split := strings.Split(repo, "/")
	if len(split) != 2 {
		return "", "", fmt.Errorf("invalid repository name: %s", repo)
	}
	return split[0], split[1], nil
}

func GetOwnerAndName() (string, string, error) {
	repo := os.Getenv("GITHUB_REPOSITORY")
	return SplitRepositoryName(repo)
}

func GetPRNumber() (int, error) {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")

	eventBytes, err := os.ReadFile(eventPath)
	if err != nil {
		return 0, err
	}

	var event map[string]interface{}
	err = json.Unmarshal(eventBytes, &event)
	if err != nil {
		return 0, err
	}
	prNumber := int(event["pull_request"].(map[string]interface{})["number"].(float64))
	return prNumber, nil
}

func PostPRComment(content string) (*github.Response, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable must be set")
	}

	owner, name, err := GetOwnerAndName()
	if err != nil {
		return nil, err
	}

	prNumber, err := GetPRNumber()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	comment := &github.PullRequestComment{
		Body: github.String(content),
	}

	_, resp, err := client.PullRequests.CreateComment(ctx, owner, name, prNumber, comment)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func main() {
	diff, err := GetGitDiffOutput()
	if err != nil {
		panic(err)
	}

	chatGPTResponse, err := GetChatGptResponse(diff)
	if err != nil {
		panic(err)
	}

	comment := fmt.Sprintf("## Review\n\n%s", string(chatGPTResponse))

	_, err = PostPRComment(comment)
	if err != nil {
		panic(err)
	}

}
