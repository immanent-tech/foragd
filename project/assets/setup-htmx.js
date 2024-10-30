// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

import htmx from 'htmx.org';
window.htmx = htmx;

// https://v1.htmx.org/docs/#logging
htmx.logger = function(elt, event, data) {
    if(console) {
        console.log(event, elt, data);
    }
}

// https://v1.htmx.org/docs/#debugging
// htmx.logAll();
