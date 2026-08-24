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
      <Preheader>Checking in about Foragd.</Preheader>

      <Container class="max-w-xl p-0 py-10 sm:p-6 prose">
        <Section class="rounded-t-lg bg-base-200 px-6 pt-6 pb-11">

          <CustomHeader />

          <Spacer class="h-36" />

          <Heading class="mb-4 text-[32px]/10 font-bold font-alegreya text-base-content">
            Your Account is Inactive
          </Heading>

          <Spacer class="h-8" />

          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
            It looks like you have signed up for a Foragd account, but haven't yet logged in to the app.
          </Text>

          <Text class="m-0 text-lg text-base-content font-alegreya-sans">
            I just wanted to check in to make sure there are no issues with your account or logging in, or any other
            troubles. Got any questions or issues, just reply to this email. Otherwise, all good.
          </Text>

        </Section>

        <Section class="bg-base-300 px-6 py-16 text-center">
          <Button
            href="/login?utm_medium=email&utm_source=notification"
            class="rounded-lg bg-primary px-7 py-3 text-sm font-bold text-primary-content"
          >
            Login
          </Button>
        </Section>

        <CustomUnsubscribe />

        <CustomFooter />
      </Container>
    </Layout>
  </Tailwind>
</template>
