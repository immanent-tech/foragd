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
      <Preheader>Welcome to Foragd.</Preheader>

      <Container class="max-w-xl p-0 py-10 sm:p-6 prose">
        <Section class="rounded-t-lg bg-base-200 px-6 pt-6 pb-11">

          <CustomHeader />

          <Spacer class="h-36" />

          <Heading class="mb-4 text-[32px]/10 font-bold font-alegreya text-primary">
            Welcome to Foragd!
          </Heading>

          <Spacer class="h-8" />

          <Text class="m-0 text-lg text-primary font-alegreya-sans">
            Thanks for starting your trial of Foragd. Enjoy no ads, no recommended content in a calm and curated
            interface.
          </Text>
        </Section>

        <Section class="bg-base-300 px-6 py-16">
          <Heading
            level="2"
            class="mb-6 text-2xl font-bold text-accent font-alegreya"
          >
            Get started
          </Heading>

          <Text class="mb-6 text-base text-neutral font-alegreya-sans">
            Go to your new home page to start subscribing to content or import your existing feeds.
          </Text>

          <Spacer class="h-4" />

          <Button
            href="/home"
            class="rounded-lg bg-primary px-7 py-3 text-sm font-bold text-primary-content"
          >
            Go to Home Page
          </Button>

          <Spacer class="h-4" />

          <Text class="text-sm/6 text-neutral font-alegreya-sans">
            Or navigate to
            <span class="hover:text-accent">https://foragd.app/home</span>
          </Text>


          <Heading
            level="2"
            class="mb-6 text-2xl font-bold text-accent font-alegreya"
          >
            Dive right in
          </Heading>

          <Text class="mb-6 text-base text-neutral font-alegreya-sans">
            Got a favorite site in mind?<br />
            Coming from another feed reader? <br />
          </Text>
          <Text class="mb-6 text-base text-secondary font-alegreya-sans">
            Get started quickly:
          </Text>
          <div class="bg-base-200 p-8 mt-4 mx-auto text-center rounded-lg text-lg font-semibold font-nunito max-w-sm">
            <Button
              href="/subscription/add?utm_medium=email&utm_source=notification"
              class="rounded-lg bg-primary px-7 py-3 text-sm font-bold text-primary-content"
            >
              Add a Subscription
            </Button>
            <Spacer class="h-4" />
            <Button
              href="/user/import?utm_medium=email&utm_source=notification"
              class="rounded-lg bg-primary px-7 py-3 text-sm font-bold text-primary-content"
            >
              Import from OPML
            </Button>
          </div>

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

        <CustomFooter />
      </Container>
    </Layout>
  </Tailwind>
</template>
