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
      <Preheader>Thank you for your purchase.</Preheader>

      <Container class="max-w-xl p-0 py-10 sm:p-6 prose">
        <Section class="rounded-t-lg bg-base-200 px-6 pt-6 pb-11">

          <CustomHeader />

          <Spacer class="h-36" />

          <Heading class="mb-4 text-[32px]/10 font-bold font-alegreya text-primary">
            Thank you for purchasing a subscription!
          </Heading>

          <Spacer class="h-8" />

          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Hey <span v-pre>{{{USER_NICKNAME}}}</span>,
          </Text>

          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            As a solo indie developer, it means so much to know that someone found Foragd useful and they are willing to
            pay for it. Your subscription will help ensure I can continue to work on it and make it even more awesome.
          </Text>

        </Section>

        <Section class="rounded-b-lg bg-base-200 px-6 pt-6 pb-11">
          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Got questions or comments? Just reply to this email.
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
