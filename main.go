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

func GitClone(owner string, repo string, token string) error {
	cloneURL := fmt.Sprintf("https://%s:%s@github.com/%s/%s", owner, token, owner, repo)
	fmt.Println(cloneURL)
	cmd := exec.Command("git", "clone", cloneURL)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func CdRepository(repo string) error {
	err := os.Chdir(repo)
	if err != nil {
		return err
	}
	return nil
}

func GitFetch() error {
	cmd := exec.Command("git", "fetch", "origin")
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func GetGitDiffOutput(headBranch string, baseBranch string, reviewIgnorePath string) ([]byte, error) {
	var patterns []string
	if _, err := os.Stat(reviewIgnorePath); !os.IsNotExist(err) {
		content, err := os.ReadFile(reviewIgnorePath)
		if err != nil {
			return nil, err
		}
		patterns = strings.Split(string(content), "\n")
	}

	cmd := exec.Command("git", "diff", fmt.Sprintf("origin/%s", headBranch), fmt.Sprintf("origin/%s", baseBranch), "--", ".")
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

func GetChatGptResponse(endpoint string, model string, apiKey string, diff []byte) ([]byte, error) {

	chatGPTRequest := ChatGPTRequest{
		Model: model,
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

func PostPRComment(owner string, repo string, prNumber int, content string, token string) (*github.Response, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	comment := &github.PullRequestComment{
		Body: github.String(content),
	}

	_, resp, err := client.PullRequests.CreateComment(ctx, owner, repo, prNumber, comment)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func main() {
	githubRepository := os.Getenv("GITHUB_REPOSITORY")
	owner, repo, err := SplitRepositoryName(githubRepository)
	if err != nil {
		panic(err)
	}

	token := os.Getenv("INPUT_GITHUB_TOKEN")
	if token == "" {
		panic("GITHUB_TOKEN environment variable must be set")
	}

	err = GitClone(owner, repo, token)
	if err != nil {
		panic(err)
	}

	err = CdRepository(repo)
	if err != nil {
		panic(err)
	}

	headBranch := os.Getenv("GITHUB_HEAD_REF")
	if headBranch == "" {
		panic("GITHUB_HEAD_REF environment variable must be set")
	}

	baseBranch := os.Getenv("GITHUB_BASE_REF")
	if baseBranch == "" {
		panic("GITHUB_BASE_REF environment variable must be set")
	}

	reviewIgnorePath := os.Getenv("INPUT_REVIEW_IGNORE_PATH")
	if reviewIgnorePath == "" {
		reviewIgnorePath = ".review-ignore"
	}

	diff, err := GetGitDiffOutput(headBranch, baseBranch, reviewIgnorePath)
	if err != nil {
		panic(err)
	}

	endpoint := "https://api.openai.com/v1/chat/completions"
	model := "gpt-3.5-turbo"
	apiKey := os.Getenv("INPUT_OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY environment variable must be set")
	}

	chatGPTResponse, err := GetChatGptResponse(endpoint, model, apiKey, diff)
	if err != nil {
		panic(err)
	}

	comment := fmt.Sprintf("## Review\n\n%s", string(chatGPTResponse))

	prNumber, err := GetPRNumber()
	if err != nil {
		panic(err)
	}

	_, err = PostPRComment(owner, repo, prNumber, comment, token)
	if err != nil {
		panic(err)
	}
}
