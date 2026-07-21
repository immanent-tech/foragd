<!--
 Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
 SPDX-License-Identifier: 	AGPL-3.0-or-later
-->
<template>
  <Tailwind>
    <template #config>
      @import "@maizzle/tailwindcss";

      @theme {
      --color-base-100: oklch(0.9858 0.0045 78.3);
      --color-base-200: oklch(0.94 0.012 82);
      --color-base-300: oklch(0.86 0.02 80.71);
      --color-base-content: oklch(0.2689 0.0165 50.73);
      --color-primary: oklch(0.48 0.0447 155.8);
      --color-primary-content: oklch(0.9858 0.0045 78.3);
      --color-secondary: oklch(0.4 0.0656 52.86);
      --color-secondary-content: oklch(0.9858 0.0045 78.3);
      --color-accent: oklch(0.42 0.0921 1.97);
      --color-accent-content: oklch(0.9858 0.0045 78.3);
      --color-neutral: oklch(0.384 0.0197 73.33);
      --color-neutral-content: oklch(0.9858 0.0045 78.3);
      --color-info: oklch(0.48 0.1434 254.62);
      --color-info-content: oklch(0.9858 0.0045 78.3);
      --color-success: oklch(0.46 0.1821 151.71);
      --color-success-content: oklch(0.9858 0.0045 78.3);
      --color-warning: oklch(0.8369 0.1644 84.43);
      --color-base-content: oklch(0.2689 0.0165 50.73);
      --color-error: oklch(0.5 0.1661 22.22);
      --color-error-content: oklch(0.9858 0.0045 78.3);
      }


    </template>
    <CustomFonts />

    <Layout class="bg-base-100">
      <Preheader>Subscribe to email newsletters with Foragd</Preheader>

      <Container class="max-w-xl p-0 py-10 sm:p-6 prose">
        <Section class="rounded-t-lg bg-base-200 px-6 pt-6 pb-11">

          <CustomHeader />

          <Spacer class="h-36" />

          <Heading class="mb-4 text-[32px]/10 font-bold font-alegreya text-primary">
            Tip: Subscribe to Email Newsletters
          </Heading>

          <Spacer class="h-8" />

          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Hey <span v-pre>{{{USER_NICKNAME}}}</span>,
          </Text>
          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Did you know you can get your email newsletters delivered to Foragd? This has many advantages:
          </Text>
        </Section>

        <Section class="bg-base-300 px-6 py-16">
          <Row class="flex flex-row sm:flex-col items-center justify-center gap-2">
            <Column class="bg-base-200 rounded-sm p-1 text-center h-64">
              <Img
                src="/content/icons/shield-lock-filled.png"
                alt="Shield with lock"
                width="64"
                height="64"
              />
              <Text class="text-sm/6 font-nunito text-primary font-semibold">
                Protect your identity.
              </Text>
              <Text class="text-sm/6 text-secondary font-alegreya-sans">
                Foragd generates a unique masked email for you to use.
              </Text>
            </Column>
            <Column class="bg-base-200 rounded-sm p-1 text-center h-64">
              <Img
                src="/content/icons/category-filled.png"
                alt="Categories icon"
                width="64"
                height="64"
              />
              <Text class="text-sm/6 font-nunito text-primary font-semibold">
                Keep organized.
              </Text>
              <Text class="text-sm/6 text-secondary font-alegreya-sans">
                Manage your email newsletters like your feed subscriptions.
              </Text>
            </Column>
            <Column class="bg-base-200 rounded-sm p-1 text-center h-64">
              <Img
                src="/content/icons/filter-filled.png"
                alt="Filter icon"
                width="64"
                height="64"
              />
              <Text class="text-sm/6 font-nunito text-primary font-semibold">
                Filter and search emails.
              </Text>
              <Text class="text-sm/6 text-secondary font-alegreya-sans">
                Filter emails and use full-text search on their contents to cut through the noise.
              </Text>
            </Column>
          </Row>

          <Heading
            level="2"
            class="mb-6 text-2xl font-bold text-accent font-alegreya"
          >
            Activate this feature:
          </Heading>

          <Text class="mb-6 text-base text-neutral font-alegreya-sans">
            <ol class="text-sm/6 text-neutral">
              <li>
                Go to your
                <a href="/user/settings?utm_medium=email&utm_source=notification">Settings</a>->Account tab.
              </li>
              <li>
                Scroll down to
                <span class="font-semibold">Subscribe to Email Newsletters</span>.
              </li>
              <li>
                Click the
                <span class="font-semibold">Generate email address</span>
                button.
              </li>
              <li>Copy the email that is displayed.</li>
            </ol>
          </Text>

          <Spacer class="h-4" />

          <Text class="text-base text-neutral font-alegreya-sans">
            Use the generated email when subscribing to newsletters and get all the benefits of content management in
            Foragd and protect your own email.
          </Text>
        </Section>

        <Section class="rounded-b-lg bg-base-200 px-6 pt-6 pb-11">
          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Also, check out the
            <Link href="/docs?utm_medium=email&utm_source=notification">help documentation</Link>
            for more information.
          </Text>
          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Got questions or comments? Just reply to this email. It goes straight to me.
          </Text>
          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Cheers,
            <br>
            Josh
          </Text>
        </Section>

        <CustomUnsubscribe />

        <CustomFooter />
      </Container>
    </Layout>
  </Tailwind>
</template>
