<!--
 Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
 SPDX-License-Identifier: 	AGPL-3.0-or-later
-->
<template>
  <Tailwind>
    <template #config>
      @import "@maizzle/tailwindcss";

      @theme {
      --color-base-100: oklch(0.995 0.002 95);
      --color-base-200: oklch(0.97 0.004 95);
      --color-base-300: oklch(0.92 0.006 95);
      --color-base-content: oklch(0.24 0.015 95);
      --color-primary: oklch(0.52 0.16 145);
      --color-primary-content: oklch(0.99 0.005 145);
      --color-secondary: oklch(0.4 0.11 152);
      --color-secondary-content: oklch(0.98 0.005 152);
      --color-accent: oklch(0.36 0.11 22);
      --color-accent-content: oklch(0.98 0.005 22);
      --color-neutral: oklch(0.3 0.01 95);
      --color-neutral-content: oklch(0.98 0.005 95);
      --color-info: oklch(0.55 0.14 245);
      --color-info-content: oklch(0.98 0.01 245);
      --color-success: oklch(0.52 0.17 148);
      --color-success-content: oklch(0.98 0.01 148);
      --color-warning: oklch(0.8 0.15 82);
      --color-warning-content: oklch(0.27 0.03 82);
      --color-error: oklch(0.52 0.19 26);
      --color-error-content: oklch(0.98 0.01 26);
      }


    </template>
    <CustomFonts />

    <Layout class="bg-base-100">
      <Preheader>Subscribe to email newsletters with Foragd</Preheader>

      <Container class="max-w-xl p-0 py-10 sm:p-6 prose">
        <Section class="rounded-t-lg bg-base-200 px-6 pt-6 pb-11">

          <CustomHeader />

          <Spacer class="h-36" />

          <Heading class="mb-4 text-[32px]/10 font-bold font-alegreya text-base-content">
            Tip: Subscribe to Email Newsletters
          </Heading>

          <Spacer class="h-8" />

          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
            Hey <span v-pre>{{{USER_NICKNAME}}}</span>,
          </Text>
          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
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
              <Text class="text-sm/6 font-nunito text-base-content font-semibold">
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
              <Text class="text-sm/6 font-nunito text-base-content font-semibold">
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
              <Text class="text-sm/6 font-nunito text-base-content font-semibold">
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
                <a
                  class="text-accent hover:text-primary"
                  href="/user/settings?utm_medium=email&utm_source=notification"
                >Settings</a>->Account tab.
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
          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
            Also, check out the
            <Link
              class="text-accent hover:text-primary"
              href="/docs?utm_medium=email&utm_source=notification"
            >help
            documentation</Link>
            for more information.
          </Text>
          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
            Got questions or comments? Just reply to this email. It goes straight to me.
          </Text>
          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
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
