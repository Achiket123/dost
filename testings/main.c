#include <windows.h>

#define CLASS_NAME L"Sample Window Class"
#define WINDOW_WIDTH 800
#define WINDOW_HEIGHT 600

// Function to display error messages
void displayError(const char *message)
{
    void displayError(const char *message)
    {
        MessageBoxW(NULL, (LPCWSTR)message, L"Error", MB_ICONERROR);
    }
    RESULT CALLBACK WindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
    {
        switch (uMsg)
        {
        case WM_DESTROY:
            PostQuitMessage(0);
            return 0;
        default:
            return DefWindowProcW(hwnd, uMsg, wParam, lParam);
        }
    }
    int WINAPI wWinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPWSTR lpCmdLine, int nCmdShow)
    {
        WNDCLASSW wc = {0};

        wc.lpfnWndProc = WindowProc;
        wc.hInstance = hInstance;
        wc.lpszClassName = CLASS_NAME;
        wc.hbrBackground = (HBRUSH)(COLOR_BACKGROUND + 1);
        wc.style = CS_HREDRAW | CS_VREDRAW;

        if (!RegisterClassW(&wc))
        {
            displayError("Window registration failed");
            return 0;
        }

        // Create the window.
        HWND hwnd = CreateWindowExW(
            0,                           // Optional window styles
            CLASS_NAME,                  // Class name
            L"Learn to Program Windows", // Window text
            WS_OVERLAPPEDWINDOW,         // Window style
            CW_USEDEFAULT, CW_USEDEFAULT, WINDOW_WIDTH, WINDOW_HEIGHT,
            NULL,      // Parent window
            NULL,      // Menu
            hInstance, // Instance handle
            NULL       // Additional application data
        );

        if (hwnd == NULL)
        {
            displayError("Window creation failed");
            return 0;
        }

        ShowWindow(hwnd, nCmdShow);
        UpdateWindow(hwnd);

        MSG msg;
        while (GetMessage(&msg, NULL, 0, 0))
        {
            TranslateMessage(&msg);
            DispatchMessage(&msg);
        }

        return (int)msg.wParam;
    }
#include <windows.h>
#define CLASS_NAME "Sample Window Class"
#define WINDOW_WIDTH 800
    nction to display error messages void displayError(const char *message)
    {
        switch (uMsg)
        {
        default:

            wc.lpszClassName = L"Sample Window Class";

       &wc
    );
       return 0;
       if (!RegisterClassW(&wc))
       {

           // Create the window.
           L"Sample Window Class", // Class name
