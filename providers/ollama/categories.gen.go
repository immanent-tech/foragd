// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package ollama

import "fmt"

// iabTier1Categories holds the 37 official Tier 1 labels from IAB Tech Lab's Content Taxonomy v3.1
// (InteractiveAdvertisingBureau/Taxonomies, Content Taxonomy 3.1.tsv). Labels are verbatim from the spec; Descriptions
// are short paraphrases written to aid zero-shot embedding classification — the taxonomy itself does not ship
// descriptions.
var iabTier1Categories = []CategoryEmbedding{
	{Label: "Attractions", Description: "amusement parks, theme parks, bars, restaurants, and things to visit"},
	{Label: "Automotive", Description: "cars, motorcycles, vehicles, personal transport"},
	{Label: "Books and Literature", Description: "novels, poetry, book reviews, authors, reading recommendations"},
	{Label: "Business and Finance", Description: "business news, corporate strategy, industry trends, economy"},
	{Label: "Careers", Description: "job searching, resumes, career advice, workplace and professional development"},
	{Label: "Communication", Description: "messaging apps, social media platforms, telecommunications"},
	{Label: "Crime", Description: "criminal activity, law enforcement, true crime, court cases"},
	{Label: "Disasters", Description: "natural disasters, accidents, emergencies, and their aftermath"},
	{Label: "Education", Description: "schools, studying, online courses, academic topics, learning resources"},
	{Label: "Entertainment", Description: "movies, music, television, celebrities, streaming media"},
	{Label: "Events", Description: "concerts, festivals, conferences, meetings, scheduled happenings"},
	{Label: "Family and Relationships", Description: "parenting, dating, marriage, family life"},
	{Label: "Fine Art", Description: "painting, sculpture, photography, art history and criticism"},
	{Label: "Food & Drink", Description: "recipes, cooking, restaurants, wine, coffee, beverages"},
	{Label: "Genres", Description: "creative content categorized by style, such as fiction genres or film genres"},
	{Label: "Healthy Living", Description: "fitness, nutrition, mental wellness, lifestyle health topics"},
	{Label: "Hobbies & Interests", Description: "crafts, collecting, board games, DIY projects, enthusiast pursuits"},
	{Label: "Holidays", Description: "seasonal and cultural holidays, holiday traditions and celebrations"},
	{Label: "Home & Garden", Description: "home improvement, interior design, gardening, DIY home projects"},
	{Label: "Law", Description: "legal topics, regulations, legal advice, court systems"},
	{Label: "Medical Health", Description: "medical conditions, treatments, healthcare, clinical topics"},
	{
		Label:       "Personal Celebrations & Life Events",
		Description: "weddings, birthdays, graduations, and other life milestones",
	},
	{Label: "Personal Finance", Description: "budgeting, saving, credit, taxes, retirement planning"},
	{Label: "Pets", Description: "dogs, cats, pet care, animal behavior and adoption"},
	{Label: "Politics", Description: "government policy, elections, political commentary and analysis"},
	{Label: "Pop Culture", Description: "internet trends, memes, celebrity gossip, viral content"},
	{
		Label:       "Technology & Computing",
		Description: "software, gadgets, programming, the internet, AI, IT, information technology, world wide web, online, open source, release notes, changelogs",
	},
	{Label: "Real Estate", Description: "buying, selling, and renting property; real estate market topics"},
	{Label: "Religion & Spirituality", Description: "faith, religious practice, meditation, spiritual growth"},
	{Label: "Science", Description: "scientific research, space, biology, physics, environment"},
	// {
	// 	Label:       "Sensitive Topics",
	// 	Description: "content flagged for brand-safety review, such as adult or controversial subject matter",
	// },
	{Label: "Shopping", Description: "product reviews, deals, e-commerce, consumer goods"},
	{Label: "Sports", Description: "athletics, sports news, teams, fitness competitions"},
	{Label: "Style & Fashion", Description: "clothing, beauty, fashion trends, personal style"},
	{Label: "Travel", Description: "destinations, trip planning, travel tips, tourism"},
	{Label: "Video Gaming", Description: "video games, esports, gaming hardware, gaming culture"},
	{Label: "War and Conflicts", Description: "military conflict, geopolitical tension, war coverage"},
}

// BuildCategories embeds each category description using an instruction tuned for classification.
func BuildCategories(categories []CategoryEmbedding) error {
	if err := LoadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	instruction := "Instruct: Classify the given text into one of the predefined categories\nQuery:"
	texts := make([]string, len(categories))
	for i, c := range categories {
		texts[i] = instruction + " " + c.Description
	}

	vectors, err := EmbedBatch(texts...)
	if err != nil {
		return fmt.Errorf("embedding categories: %w", err)
	}
	for i := range categories {
		categories[i].Embedding = vectors[i]
	}
	return nil
}
