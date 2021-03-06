package sendgrid

type SGMailV3 struct {
	From             *Email             `json:"from,omitempty"`
	Subject          string             `json:"subject,omitempty"`
	Personalizations []*Personalization `json:"personalization,omitempty"`
	Content          []*Content         `json:"content,omitempty"`
	Attachments      []*Attachment      `json:"attachments,omitempty"`
	TemplateID       string             `json:"template_id,omitempty"`
	Sections         map[string]string  `json:"sections,omitempty"`
	Headers          map[string]string  `json:"headers,omitempty"`
	Categories       []string           `json:"categories,omitempty"`
	CustomArgs       map[string]string  `json:"custom_args,omitempty"`
	SendAt           int                `json:"send_at,omitempty"`
	BatchID          string             `json:"batch_id,omitempty"`
	Asm              *Asm               `json:"asm,omitempty"`
	IPPoolID         int                `json:"ip_pool_id,omitempty"`
	MailSettings     *MailSettings      `json:"mail_settings,omitempty"`
	TrackingSettings *TrackingSettings  `json:"tracking_settings,omitempty"`
}

type Personalization struct {
	To            []*Email          `json:"to,omitempty"`
	CC            []*Email          `json:"cc,omitempty"`
	BCC           []*Email          `json:"bcc,omitempty"`
	Subject       string            `json:"subject,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Substitutions map[string]string `json:"substitutions,omitempty"`
	CustomArgs    map[string]string `json:"custom_args,omitempty"`
	Categories    []string          `json:"categories,omitempty"`
	SendAt        int               `json:"send_at,omitempty"`
}

type Email struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"email,omitempty"`
}

type Content struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type Attachment struct {
	Content     string `json:"content,omitempty"`
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	ContentID   string `json:"content_id,omitempty"`
}

type Asm struct {
	GroupID         int   `json:"group_id,omitempty"`
	GroupsToDisplay []int `json:"groups_to_display,omitempty"`
}

type MailSettings struct {
	BCC                  *BccSetting    `json:"bcc,omitempty"`
	BypassListManagement *Setting       `json:"bypass_list_management,omitempty"`
	Footer               *FooterSetting `json:"footer,omitempty"`
	SandboxMode          *Setting       `json:"sandbox_mode,omitempty"`
}

type TrackingSettings struct {
	ClickTracking        *ClickTrackingSetting        `json:"click_tracking,omitempty"`
	OpenTracking         *OpenTrackingSetting         `json:"open_tracking,omitempty"`
	SubscriptionTracking *SubscriptionTrackingSetting `json:"subscription_tracking,omitempty"`
	GoogleAnalytics      *GaSetting                   `json:"ganalytics,omitempty"`
	BCC                  *BccSetting                  `json:"bcc,omitempty"`
	BypassListManagement *Setting                     `json:"bypass_list_management,omitempty"`
	Footer               *FooterSetting               `json:"footer,omitempty"`
	SandboxMode          *SandboxModeSetting          `json:"sandbox_mode,omitempty"`
}

type BccSetting struct {
	Enable bool   `json:"enable,omitempty"`
	Email  *Email `json:"email,omitempty"`
}

type FooterSetting struct {
	Enable bool   `json:"enable,omitempty"`
	Text   string `json:"text,omitempty"`
	Html   string `json:"html,omitempty"`
}

type ClickTrackingSetting struct {
	Enable     bool `json:"enable,omitempty"`
	EnableText bool `json:"enable_text,omitempty"`
}

type OpenTrackingSetting struct {
	Enable          bool   `json:"enable,omitempty"`
	SubstitutionTag string `json:"substitution_tag,omitempty"`
}

type SandboxModeSetting struct {
	Enable      bool              `json:"enable,omitempty"`
	ForwardSpam bool              `json:"forward_spam,omitempty"`
	SpamCheck   *SpamCheckSetting `json:"spam_check,omitempty"`
}

type SpamCheckSetting struct {
	Enable        bool   `json:"enable,omitempty"`
	SpamThreshold int    `json:"spam_threshold,omitempty"`
	PostToURL     string `json:"post_to_url,omitempty"`
}

type SubscriptionTrackingSetting struct {
	Enable          bool   `json:"enable,omitempty"`
	Text            string `json:"text,omitempty"`
	Html            string `json:"html,omitempty"`
	SubstitutionTag string `json:"substitution_tag,omitempty"`
}

type GaSetting struct {
	Enable          bool   `json:"enable,omitempty"`
	CampaignSource  string `json:"Campaign Source,omitempty"`
	CampaignTerm    string `json:"Campaign Term,omitempty"`
	CampaignContent string `json:"Campaign Content,omitempty"`
	CampaignName    string `json:"Campaign Name,omitempty"`
}

type Setting struct {
	Enable bool `json:"enable,omitempty"`
}

func NewV3Mail() *SGMailV3 {
	return &SGMailV3{
		Personalizations: make([]*Personalization, 0),
		Content:          make([]*Content, 0),
		Attachments:      make([]*Attachment, 0),
	}
}

func (s *SGMailV3) AddPersonalizations(p ...*Personalization) *SGMailV3 {
	if s.Personalizations == nil {
		s.Personalizations = make([]*Personalization, 0)
	}
	s.Personalizations = append(s.Personalizations, p...)

	return s
}

func (s *SGMailV3) SetFrom(e *Email) *SGMailV3 {
	s.From = e
	return s
}

func (s *SGMailV3) SetTemplateID(templateID string) *SGMailV3 {
	s.TemplateID = templateID
	return s
}

func (s *SGMailV3) AddSection(key string, value string) *SGMailV3 {
	if s.Sections == nil {
		s.Sections = make(map[string]string)
	}

	s.Sections[key] = value
	return s
}

func (s *SGMailV3) SetHeader(key string, value string) *SGMailV3 {
	if s.Headers == nil {
		s.Headers = make(map[string]string)
	}

	s.Headers[key] = value
	return s
}

func (s *SGMailV3) AddCategories(category ...string) *SGMailV3 {
	if s.Categories == nil {
		s.Categories = make([]string, 0)
	}

	s.Categories = append(s.Categories, category...)
	return s
}

func (s *SGMailV3) SetCustomArg(key string, value string) *SGMailV3 {
	if s.CustomArgs == nil {
		s.CustomArgs = make(map[string]string)
	}

	s.CustomArgs[key] = value
	return s
}

func (s *SGMailV3) SetSendAt(sendAt int) *SGMailV3 {
	s.SendAt = sendAt
	return s
}

func (s *SGMailV3) SetBatchID(batchID string) *SGMailV3 {
	s.BatchID = batchID
	return s
}

func (s *SGMailV3) SetASM(asm *Asm) *SGMailV3 {
	s.Asm = asm
	return s
}

func (s *SGMailV3) SetIPPoolID(ipPoolID int) *SGMailV3 {
	s.IPPoolID = ipPoolID
	return s
}

func (s *SGMailV3) SetMailSettings(mailSettings *MailSettings) *SGMailV3 {
	s.MailSettings = mailSettings
	return s
}

func (s *SGMailV3) SetTrackingSettings(trackingSettings *TrackingSettings) *SGMailV3 {
	s.TrackingSettings = trackingSettings
	return s
}

func NewPersonalization() *Personalization {
	return &Personalization{
		To:            make([]*Email, 0),
		CC:            make([]*Email, 0),
		BCC:           make([]*Email, 0),
		Headers:       make(map[string]string),
		Substitutions: make(map[string]string),
		CustomArgs:    make(map[string]string),
		Categories:    make([]string, 0),
	}
}

func (p *Personalization) AddTos(to ...*Email) {
	p.To = append(p.To, to...)
}

func (p *Personalization) AddCCs(cc ...*Email) {
	p.CC = append(p.CC, cc...)
}

func (p *Personalization) AddBCCs(bcc ...*Email) {
	p.BCC = append(p.BCC, bcc...)
}

func (p *Personalization) SetHeader(key string, value string) {
	p.Headers[key] = value
}

func (p *Personalization) SetSubstitution(key string, value string) {
	p.Substitutions[key] = value
}

func (p *Personalization) SetCustomArg(key string, value string) {
	p.CustomArgs[key] = value
}

func (p *Personalization) SetSendAt(sendAt int) {
	p.SendAt = sendAt
}

func NewAttachment() *Attachment {
	return &Attachment{}
}

func (a *Attachment) SetContent(content string) *Attachment {
	a.Content = content
	return a
}

func (a *Attachment) SetType(contentType string) *Attachment {
	a.Type = contentType
	return a
}

func (a *Attachment) SetFilename(filename string) *Attachment {
	a.Filename = filename
	return a
}

func (a *Attachment) SetDisposition(disposition string) *Attachment {
	a.Disposition = disposition
	return a
}

func (a *Attachment) SetContentID(contentID string) *Attachment {
	a.ContentID = contentID
	return a
}

func NewASM() *Asm {
	return &Asm{}
}

func (a *Asm) SetGroupID(groupID int) *Asm {
	a.GroupID = groupID
	return a
}

func (a *Asm) AddGroupsToDisplay(groupsToDisplay ...int) *Asm {
	if a.GroupsToDisplay == nil {
		a.GroupsToDisplay = make([]int, 0)
	}

	a.GroupsToDisplay = append(a.GroupsToDisplay, groupsToDisplay...)
	return a
}

func NewMailSettings() *MailSettings {
	return &MailSettings{}
}

func (m *MailSettings) SetBCC(bcc *BccSetting) *MailSettings {
	m.BCC = bcc
	return m
}

func (m *MailSettings) SetBypassListManagement(bypassListManagement *Setting) *MailSettings {
	m.BypassListManagement = bypassListManagement
	return m
}

func (m *MailSettings) SetFooter(footerSetting *FooterSetting) *MailSettings {
	m.Footer = footerSetting
	return m
}

func (m *MailSettings) SetSandboxMode(sandboxMode *Setting) *MailSettings {
	m.SandboxMode = sandboxMode
	return m
}

func NewTrackingSettings() *TrackingSettings {
	return &TrackingSettings{}
}

func (t *TrackingSettings) SetClickTracking(clickTracking *ClickTrackingSetting) *TrackingSettings {
	t.ClickTracking = clickTracking
	return t

}

func (t *TrackingSettings) SetOpenTracking(openTracking *OpenTrackingSetting) *TrackingSettings {
	t.OpenTracking = openTracking
	return t
}

func (t *TrackingSettings) SetSubscriptionTracking(subscriptionTracking *SubscriptionTrackingSetting) *TrackingSettings {
	t.SubscriptionTracking = subscriptionTracking
	return t
}

func (t *TrackingSettings) SetGoogleAnalytics(googleAnalytics *GaSetting) *TrackingSettings {
	t.GoogleAnalytics = googleAnalytics
	return t
}

func NewBCCSetting() *BccSetting {
	return &BccSetting{}
}

func (b *BccSetting) SetEnable(enable bool) *BccSetting {
	b.Enable = enable
	return b
}

func (b *BccSetting) SetEmail(email *Email) *BccSetting {
	b.Email = email
	return b
}

func NewFooterSetting() *FooterSetting {
	return &FooterSetting{}
}

func (f *FooterSetting) SetEnable(enable bool) *FooterSetting {
	f.Enable = enable
	return f
}

func (f *FooterSetting) SetText(text string) *FooterSetting {
	f.Text = text
	return f
}

func (f *FooterSetting) SetHTML(html string) *FooterSetting {
	f.Html = html
	return f
}

func NewOpenTrackingSetting() *OpenTrackingSetting {
	return &OpenTrackingSetting{}
}

func (o *OpenTrackingSetting) SetEnable(enable bool) *OpenTrackingSetting {
	o.Enable = enable
	return o
}

func (o *OpenTrackingSetting) SetSubstitutionTag(subTag string) *OpenTrackingSetting {
	o.SubstitutionTag = subTag
	return o
}

func NewSubscriptionTrackingSetting() *SubscriptionTrackingSetting {
	return &SubscriptionTrackingSetting{}
}

func (s *SubscriptionTrackingSetting) SetEnable(enable bool) *SubscriptionTrackingSetting {
	s.Enable = enable
	return s
}

func (s *SubscriptionTrackingSetting) SetText(text string) *SubscriptionTrackingSetting {
	s.Text = text
	return s
}

func (s *SubscriptionTrackingSetting) SetHTML(html string) *SubscriptionTrackingSetting {
	s.Html = html
	return s
}

func (s *SubscriptionTrackingSetting) SetSubstitutionTag(subTag string) *SubscriptionTrackingSetting {
	s.SubstitutionTag = subTag
	return s
}

func NewGaSetting() *GaSetting {
	return &GaSetting{}
}

func (g *GaSetting) SetEnable(enable bool) *GaSetting {
	g.Enable = enable
	return g
}

func (g *GaSetting) SetCampaignSource(campaignSource string) *GaSetting {
	g.CampaignSource = campaignSource
	return g
}

func (g *GaSetting) SetCampaignContent(campaignContent string) *GaSetting {
	g.CampaignContent = campaignContent
	return g
}

func (g *GaSetting) SetCampaignTerm(campaignTerm string) *GaSetting {
	g.CampaignTerm = campaignTerm
	return g
}

func (g *GaSetting) SetCampaignName(campaignName string) *GaSetting {
	g.CampaignName = campaignName
	return g
}

func NewSetting(enable bool) *Setting {
	return &Setting{Enable: enable}
}

func NewEmail(name string, address string) *Email {
	return &Email{
		Name:    name,
		Address: address,
	}
}

func NewClickTrackingSetting(enable bool, enableText bool) *ClickTrackingSetting {
	return &ClickTrackingSetting{
		Enable:     enable,
		EnableText: enableText,
	}
}

func NewSpamCheckSetting(enable bool, spamThreshold int, postToURL string) *SpamCheckSetting {
	return &SpamCheckSetting{
		Enable:        enable,
		SpamThreshold: spamThreshold,
		PostToURL:     postToURL,
	}
}

func NewSandboxModeSetting(enable bool, forwardSpam bool, spamCheck *SpamCheckSetting) *SandboxModeSetting {
	return &SandboxModeSetting{
		Enable:      enable,
		ForwardSpam: forwardSpam,
		SpamCheck:   spamCheck,
	}
}
