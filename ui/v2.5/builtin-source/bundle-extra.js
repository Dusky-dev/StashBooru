(function () {
    const CARD_ID = "bundle-extra";

    const fields = [
        ["birthdate", "Birthdate"],
        ["death_date", "Death Date"],
        ["country", "Country"],
        ["ethnicity", "Ethnicity"],
        ["hair_color", "Hair Color"],
        ["eye_color", "Eye Color"],
        ["height_cm", "Height (cm)"],
 ["weight", "Weight (kg)"],
 ["penis_length", "Penis Length (cm)"],
 ["circumcised", "Circumcised"],
 ["measurements", "Measurements"],
 ["fake_tits", "Fake Tits"],
 ["tattoos", "Tattoos"],
 ["piercings", "Piercings"],
 ["career_start", "Career Start"],
 ["career_end", "Career End"],
 ["stash_ids", "Stash IDs"],

 // Studio fields
 ["parent_id", "Parent Studio"],
 ["child_ids", "Subsidiary Studios"]
    ];

    function findField(fieldName, expectedLabel) {
        const candidates = document.querySelectorAll(
            `[data-field="${fieldName}"]`
        );

        for (const element of candidates) {
            const label = element.querySelector("label");

            if (!label) {
                continue;
            }

            if (label.textContent.trim() === expectedLabel) {
                return element;
            }
        }

        return null;
    }

    function getAvailableFields() {
        return fields
        .map(function (field) {
            return {
                name: field[0],
                label: field[1],
                element: findField(field[0], field[1])
            };
        })
        .filter(function (field) {
            return field.element !== null;
        });
    }

    function removeCard() {
        const card = document.getElementById(CARD_ID);

        if (card) {
            card.remove();
        }
    }

    function createCard() {
        if (document.getElementById(CARD_ID)) {
            return;
        }

        const customFields = document.querySelector(".custom-fields-input");

        if (!customFields) {
            return;
        }

        const availableFields = getAvailableFields();

        if (availableFields.length === 0) {
            return;
        }

        const card = document.createElement("div");

        card.id = CARD_ID;
        card.className = "custom-fields-input";

        card.innerHTML = `
        <div class="collapse-header">
        <button
        type="button"
        class="minimal collapse-button btn btn-primary"
        aria-expanded="false"
        >
        <span class="character-details-arrow">
        <svg
        data-prefix="fas"
        data-icon="chevron-right"
        class="svg-inline--fa fa-chevron-right fa-fw fa-icon"
        role="img"
        viewBox="0 0 320 512"
        aria-hidden="true"
        >
        <path
        fill="currentColor"
        d="M311.1 233.4c12.5 12.5 12.5 32.8 0 45.3l-192 192c-12.5 12.5-32.8 12.5-45.3 0s-12.5-32.8 0-45.3L243.2 256 73.9 86.6c-12.5-12.5-12.5-32.8 0-45.3s32.8-12.5 45.3 0l192 192z"
        ></path>
        </svg>
        </span>
        <span>Advanced</span>
        </button>
        </div>

        <div class="character-details-collapse">
        <div class="character-extra-fields"></div>
        </div>
        `;

        customFields.parentNode.insertBefore(card, customFields);

        const button = card.querySelector(".collapse-button");
        const collapse = card.querySelector(".character-details-collapse");
        const container = card.querySelector(".character-extra-fields");
        const icon = card.querySelector(".character-details-arrow");

        availableFields.forEach(function (field) {
            container.appendChild(field.element);
        });

        collapse.style.display = "block";
        collapse.style.height = "0px";
        collapse.style.overflow = "hidden";
        collapse.style.opacity = "0";
        collapse.style.transition =
        "height 0.35s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s ease";

        icon.style.display = "inline-block";
        icon.style.transition =
        "transform 0.2s cubic-bezier(0.4, 0, 0.2, 1)";
        icon.style.transformOrigin = "center";
        icon.style.transform = "rotate(0deg)";

        button.addEventListener("click", function () {
            const isOpen =
            button.getAttribute("aria-expanded") === "true";

            if (isOpen) {
                const height = collapse.scrollHeight;

                collapse.style.height = height + "px";
                collapse.style.opacity = "1";

                collapse.offsetHeight;

                requestAnimationFrame(function () {
                    collapse.style.height = "0px";
                    collapse.style.opacity = "0";
                });

                button.setAttribute("aria-expanded", "false");
                icon.style.transform = "rotate(0deg)";
            } else {
                collapse.style.display = "block";
                collapse.style.height = "0px";
                collapse.style.opacity = "0";

                collapse.offsetHeight;

                const height = collapse.scrollHeight;

                requestAnimationFrame(function () {
                    collapse.style.height = height + "px";
                    collapse.style.opacity = "1";
                });

                button.setAttribute("aria-expanded", "true");
                icon.style.transform = "rotate(90deg)";
            }
        });

        collapse.addEventListener("transitionend", function (event) {
            if (event.propertyName !== "height") {
                return;
            }

            if (
                button.getAttribute("aria-expanded") === "true"
            ) {
                collapse.style.height = "auto";
            }
        });
    }

    function init() {
        const customFields = document.querySelector(".custom-fields-input");

        if (!customFields) {
            removeCard();
            return;
        }

        const availableFields = getAvailableFields();

        if (availableFields.length === 0) {
            removeCard();
            return;
        }

        createCard();
    }

    let observerTimer = null;

    const observer = new MutationObserver(function () {
        clearTimeout(observerTimer);

        observerTimer = setTimeout(function () {
            init();
        }, 50);
    });

    observer.observe(document.body, {
        childList: true,
        subtree: true
    });

    init();
})();
