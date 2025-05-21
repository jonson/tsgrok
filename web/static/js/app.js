// Placeholder for future JavaScript
console.log("Inspector app.js loaded");

document.addEventListener('DOMContentLoaded', () => {
    const requestsLogPane = document.querySelector('.requests-log-pane');
    const requestDetailsContentWrapper = document.getElementById('request-details-content-wrapper');

    // Handle selection of request items in the left pane
    if (requestsLogPane) {
        requestsLogPane.addEventListener('click', (event) => {
            const targetItem = event.target.closest('.request-item');
            if (!targetItem) {
                return; // Click wasn't on a request-item or its child
            }

            // Remove .selected-request from previously selected item
            const currentlySelected = requestsLogPane.querySelector('.request-item.selected-request');
            if (currentlySelected) {
                currentlySelected.classList.remove('selected-request');
            }

            // Add .selected-request to the clicked item
            targetItem.classList.add('selected-request');
            
            // Hide initial message if it's still there
            const initialMessage = document.getElementById('initial-detail-message');
            if (initialMessage) {
                initialMessage.style.display = 'none';
            }
        });
    }

    // Handle tab switching for content loaded by HTMX
    if (requestDetailsContentWrapper) {
        requestDetailsContentWrapper.addEventListener('htmx:afterSwap', function(event) {
            // Content has been swapped in by HTMX, re-initialize tabs for the new content
            // The new content is event.detail.target
            const newContent = event.detail.target.querySelector('.details-container') || event.detail.target;
            initializeTabs(newContent);
        });

        // Also initialize tabs if content is already there on page load (e.g., if first item is auto-loaded by server)
        // Or if HTMX is not used for the very first load but pre-renders one detail.
        const existingDetailsContainer = requestDetailsContentWrapper.querySelector('.details-container');
        if (existingDetailsContainer) {
             initializeTabs(existingDetailsContainer);
        }
    }
});

function initializeTabs(container) {
    if (!container) return;

    const tabButtons = container.querySelectorAll('.details-tabs .tab-button');
    const tabContents = container.querySelectorAll('.tab-content-area .tab-detail-content');

    if (tabButtons.length === 0) return; // No tabs to initialize

    tabButtons.forEach(button => {
        button.addEventListener('click', () => {
            activateTab(button, tabButtons, container); 
        });
    });

    // Ensure the default active tab (set by server with .active class) is shown
    // Or activate the first one if none are marked active in HTML
    let activeTabButton = container.querySelector('.details-tabs .tab-button.active');
    if (!activeTabButton && tabButtons.length > 0) {
        activeTabButton = tabButtons[0]; // Default to first tab if none are active
    }
    
    if (activeTabButton) {
        activateTab(activeTabButton, tabButtons, container);
    } else {
        // Hide all content if no tabs somehow, or log error
        tabContents.forEach(content => content.style.display = 'none');
    }
}

function activateTab(activeButton, allTabButtons, container) {
    allTabButtons.forEach(btn => btn.classList.remove('active'));
    activeButton.classList.add('active');

    const targetContentId = activeButton.dataset.tabTarget;
    
    // Hide all tab content within this specific container
    const currentTabContents = container.querySelectorAll('.tab-content-area .tab-detail-content');
    currentTabContents.forEach(content => {
        content.style.display = 'none';
    });
    
    // Show the target one
    const activeContentElement = container.querySelector(`#${targetContentId}`);
    if (activeContentElement) {
        activeContentElement.style.display = 'block';
    } else {
        console.warn(`Tab content area with ID '${targetContentId}' not found within the current detail container.`);
    }
}

// Delegated event listener for copy URL buttons
document.addEventListener('click', function(event) {
    if (event.target.matches('.copy-url-button')) {
        const urlToCopy = event.target.dataset.url;
        if (urlToCopy) {
            navigator.clipboard.writeText(urlToCopy).then(() => {
                // Optional: Provide user feedback (e.g., change text, tooltip)
                const originalText = event.target.textContent;
                event.target.textContent = '✅'; // Temporary feedback
                setTimeout(() => {
                    event.target.textContent = originalText;
                }, 1500); // Revert after 1.5 seconds
            }).catch(err => {
                console.error('Failed to copy URL: ', err);
                // Optional: Alert user or provide other error feedback
                alert('Failed to copy URL. See console for details.');
            });
        }
    }
}); 