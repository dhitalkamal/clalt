package dashboard

func signInPrompt() rich { return prompt("Signed out. Press ", "s", " to sign in.") }

// signedOutContent is what every view shows for a signed out account
func signedOutContent(c viewCtx) content {
	return content{body: wrap(signInPrompt(), c.w)}
}

func emptyContent(w int, parts ...string) content {
	return content{body: wrap(prompt(parts...), w)}
}
