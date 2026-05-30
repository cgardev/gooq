// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	// The site is published as a GitHub Pages project site at
	// https://cgardev.github.io/gooq/, so the origin and the base path
	// are configured separately.
	site: 'https://cgardev.github.io',
	base: '/gooq',
	integrations: [
		starlight({
			title: 'gooq',
			description:
				'A type-safe, fluent, zero-dependency SQL query builder for Go inspired by jOOQ.',
			// The header logo and the browser tab icon both use the SQL file
			// artwork. The logo is resolved relative to the project root, while the
			// favicon is served from the public directory.
			logo: {
				src: './src/assets/sql-file.png',
				alt: 'gooq',
				replacesTitle: false,
			},
			favicon: '/sql-file.png',
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/cgardev/gooq',
				},
			],
			// Surface an "Edit page" link pointing at the documentation sources
			// within the repository.
			editLink: {
				baseUrl: 'https://github.com/cgardev/gooq/edit/main/docs/',
			},
			sidebar: [
				{
					label: 'Start Here',
					items: [
						{ label: 'Introduction', link: '/' },
						{ label: 'Getting Started', slug: 'getting-started' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'Building Queries', slug: 'guides/building-queries' },
						{
							label: 'Predicates & Conditions',
							slug: 'guides/predicates-and-conditions',
						},
						{
							label: 'Inserts, Updates & Deletes',
							slug: 'guides/inserts-updates-deletes',
						},
						{ label: 'Dialects', slug: 'guides/dialects' },
						{ label: 'Code Generation', slug: 'guides/code-generation' },
					],
				},
				{
					label: 'Reference',
					items: [{ label: 'API Reference', slug: 'reference/api' }],
				},
			],
		}),
	],
});
