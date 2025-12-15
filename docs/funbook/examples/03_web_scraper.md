# 03. Web Scraper

## Task
Download web pages, parse HTML, extract data.

---

## Simple scraper

```rust
import "lib/http" (httpGet)
import "lib/regex" (regexFindAll, regexCapture, regexReplaceAll)
import "lib/string" (stringTrim)

// Extract all links from a page
fun extractLinks(html: String) -> List<String> {
    regexFindAll("<a[^>]+href=\"([^\"]+)\"", html)
}

// Extract title
fun extractTitle(html: String) -> String {
    match regexCapture("<title>([^<]+)</title>", html) {
        Some(groups) -> stringTrim(groups[1])
        Zero -> "No title"
    }
}

// Extract all text (remove tags)
fun stripTags(html: String) -> String {
    h1 = regexReplaceAll("<[^>]+>", "", html)
    h2 = regexReplaceAll("\\s+", " ", h1)
    stringTrim(h2)
}

fun main() {
    match httpGet("https://example.com") {
        Ok(resp) -> {
            print("Status: " ++ show(resp.status))
            print("Title: " ++ extractTitle(resp.body))
            print("Links found: " ++ show(len(extractLinks(resp.body))))
        }
        Fail(e) -> print("Error: " ++ e)
    }
}

main()

```

---

## Parsing structured data

### Extracting product list

```rust
import "lib/http" (httpGet)
import "lib/regex" (regexFindAll, regexCapture)
import "lib/string" (stringTrim)
import "lib/list" (map)

type Product = {
    name: String,
    price: String,
    link: String
}

fun parseProducts(html: String) -> List<Product> {
    // Find all product blocks
    productPattern = "<div class=\"product\">(.*?)</div>"
    blocks = regexFindAll(productPattern, html)
    
    map(fun(block) -> {
        name = match regexCapture("<h2[^>]*>([^<]+)</h2>", block) {
            Some(g) -> stringTrim(g[1])
            Zero -> "Unknown"
        }
        
        price = match regexCapture("<span class=\"price\">([^<]+)</span>", block) {
            Some(g) -> stringTrim(g[1])
            Zero -> "N/A"
        }
        
        link = match regexCapture("<a href=\"([^\"]+)\"", block) {
            Some(g) -> g[1]
            Zero -> ""
        }
        
        { name: name, price: price, link: link }
    }, blocks)
}

```

---

## API Scraping (JSON)

```rust
import "lib/http" (httpGet)
import "lib/json" (jsonDecode)
import "lib/list" (map, sortBy, take)

type Repo = {
    name: String,
    stars: Int,
    language: String
}

fun getGitHubRepos(username: String) -> Result<String, List<Repo>> {
    url = "https://api.github.com/users/" ++ username ++ "/repos"
    
    match httpGet(url) {
        Ok(resp) -> match jsonDecode(resp.body) {
            Ok(repos) -> Ok(map(fun(r) -> {
                name: r.name,
                stars: r.stargazers_count,
                language: r.language
            }, repos))
            Fail(e) -> Fail("Parse error: " ++ e)
        }
        Fail(e) -> Fail("HTTP error: " ++ e)
    }
}

fun main() {
    match getGitHubRepos("torvalds") {
        Ok(repos) -> {
            print("Top repos by stars:\n")
            sorted = sortBy(repos, fun(a, b) -> b.stars - a.stars)
            for repo in take(sorted, 10) {
                print(show(repo.stars) ++ " - " ++ repo.name ++ " (" ++ repo.language ++ ")")
            }
        }
        Fail(e) -> print("Error: " ++ e)
    }
}

main()

```

---

## Rate Limiting

```rust
import "lib/time" (sleepMs)
import "lib/http" (httpGet)

// Limit: N requests per second
fun rateLimitedFetch(urls: List<String>, requestsPerSecond: Int) -> List<Result<String, String>> {
    delay = 1000 / requestsPerSecond
    results = []
    
    for url in urls {
        match httpGet(url) {
            Ok(resp) -> results = results ++ [Ok(resp.body)]
            Fail(e) -> results = results ++ [Fail(e)]
        }
        sleepMs(delay)
    }
    
    results
}

```

---

## Saving results

```rust
import "lib/io" (fileWrite)
import "lib/json" (jsonEncode)
import "lib/path" (pathJoin)

type CrawlResult = {
    url: String,
    title: String,
    status: Int
}

// Export to JSON
fun exportResults(results: List<CrawlResult>, filename: String) {
    json = jsonEncode(results)
    match fileWrite(filename, json) {
        Ok(_) -> print("Exported to " ++ filename)
        Fail(e) -> print("Export error: " ++ e)
    }
}

```

---

## Error handling and retry

```rust
import "lib/http" (httpGet)
import "lib/time" (sleepMs)

fun fetchWithRetry(url: String, maxRetries: Int, delayMs: Int) -> Result<String, String> {
    fun attempt(n: Int) -> Result<String, String> {
        if n > maxRetries { 
            Fail("Max retries exceeded")
        } else {
            match httpGet(url) {
                Ok(resp) -> {
                    if resp.status == 200 { Ok(resp.body) } 
                    else {
                        print("Retry " ++ show(n) ++ "/" ++ show(maxRetries) ++ " for " ++ url)
                        sleepMs(delayMs * n)
                        attempt(n + 1)
                    }
                }
                Fail(e) -> {
                    print("Retry " ++ show(n) ++ "/" ++ show(maxRetries) ++ ": " ++ e)
                    sleepMs(delayMs * n)
                    attempt(n + 1)
                }
            }
        }
    }
    attempt(1)
}

```

---

## Best Practices

1. Rate limiting - don't overload the server
2. User-Agent - set correct User-Agent
3. Robots.txt - respect site rules
4. Caching - don't download the same page twice
5. Error handling - graceful error handling
6. Timeout - set timeouts for requests
